package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/edatts/dstore/p2p"
)

func init() {
	gob.Register(PayloadData_Notify_NewFile{})
	gob.Register(PayloadData_RequestFile{})
}

type ServerOpts struct {
	StorageRoot    string
	Transport      p2p.Transport
	BootstrapNodes []string
}

type Server struct {
	ServerOpts

	pendingFiles *set[string]
	peers        *peerMap
	store        *Store
	quitCh       chan struct{}
}

func NewServer(opts ServerOpts) *Server {
	storeOpts := StoreOpts{
		StorageRoot: opts.StorageRoot,
	}

	return &Server{
		ServerOpts: opts,

		pendingFiles: newSet[string](),
		peers: &peerMap{
			m:       make(map[net.Addr]p2p.Peer),
			RWMutex: sync.RWMutex{},
		},
		store:  NewStore(storeOpts),
		quitCh: make(chan struct{}),
	}
}

func (s *Server) startBootstrap() error {
	if len(s.BootstrapNodes) == 0 {
		return nil
	}
	for _, addr := range s.BootstrapNodes {
		if len(addr) == 0 {
			continue
		}
		go func(addr string) {
			if err := s.Transport.Dial(addr); err != nil {
				log.Printf("error dialing bootstrap node: %s", err)
			}
		}(addr)
	}

	return nil
}

func (s *Server) Start() error {
	if err := s.Transport.ListenAndAccept(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	go s.startBootstrap()

	s.StartMessageLoop()

	return nil
}

func (s *Server) Stop() {
	s.quitCh <- struct{}{}
}

func (s *Server) StartMessageLoop() {
	for {
		select {
		case msg := <-s.Transport.MsgChan():
			log.Printf("(%s): Received message from: %s", s.Transport.LAddr(), msg.From)
			// Here we need to process each messaged, workflow for that is
			// as follows:
			// 1. Peer will halt it's read loop with a wg.
			// 2. We handle the message.
			// 3. Carry out work associated with Payload.
			// 4. We free the peers read loop using the wg.

			if err := s.handleMessage(&msg); err != nil {
				log.Printf("Failed to process message: %s\n", err)
			}

			// log.Println("Message handled...")

			p, ok := s.peers.Get(msg.From)
			if !ok {
				log.Println("Could not find peer in peers map.")
				continue
			}

			// log.Println("Resuming peer read loop...")

			p.WaitGroup().Done()

		case <-s.quitCh:
			log.Println("Message received on quit channel, stopping server.")
			s.Transport.Close()
			// Should clean up peer conns as well.
			return
		}
	}
}

// Stream data to peers that don't already have the file.
func (s *Server) BroadcastData(r io.ReadCloser) error {
	peers := []io.Writer{}
	for _, peer := range s.peers.Values() {
		peers = append(peers, peer)
	}

	mw := io.MultiWriter(peers...)

	_, err := io.Copy(mw, r)
	if err != nil {
		return fmt.Errorf("failed to copy data to peers: %w", err)
	}

	if err = r.Close(); err != nil {
		return fmt.Errorf("failed closing read closer: %w", err)
	}

	return nil
}

func (s *Server) BroadcastMessage(m *p2p.Message) error {
	for _, peer := range s.peers.Values() {
		if err := peer.Send(m.Payload); err != nil {
			return fmt.Errorf("failed to send payload to peers: %w", err)
		}
	}
	return nil
}

func (s *Server) handleMessage(msg *p2p.Message) error {
	r := bytes.NewReader(msg.Payload)
	payload := Payload{}
	if err := gob.NewDecoder(r).Decode(&payload); err != nil {
		return fmt.Errorf("failed to decode payload: %w", err)
	}

	switch payload.Type {
	case PayloadType_Notify_NewFile:
		// Get file hash and metadata
		data := payload.Data.(PayloadData_Notify_NewFile)
		fileHash := data.FileHash

		// Check if we have file.
		exists, err := s.store.fileExists(fileHash)
		if err != nil {
			return err
		}

		// If we have the file, return
		if exists {
			log.Printf("(%s): File already present on disk.", s.Transport.LAddr())
			return nil
		}

		// If we are already writing the file, return
		if s.pendingFiles.Has(fileHash) {
			log.Printf("(%s): File already pending.", s.Transport.LAddr())
			return nil
		}

		// Otherwise, request the file from the peer that notified us.
		encoded := new(bytes.Buffer)
		payload := Payload{
			Type: PayloadType_RequestFile,
			Data: PayloadData_RequestFile{
				FileHash: fileHash,
			},
		}

		err = gob.NewEncoder(encoded).Encode(payload)
		if err != nil {
			return err
		}

		peer, _ := s.peers.Get(msg.From)
		err = peer.Send(encoded.Bytes())
		if err != nil {
			return err
		}

		s.pendingFiles.Add(fileHash)
		defer s.pendingFiles.Remove(fileHash)

		// Now write the peers response to disk.

		hash, err := s.StoreData(io.LimitReader(peer, data.Metadata.FileSize))
		if err != nil {
			// We might need to clean up tmp/ here as well.
			return fmt.Errorf("failed to store file: %w", err)
		}

		// hash, err := s.store.Write(io.LimitReader(peer, data.Metadata.FileSize))
		// if err != nil {
		// 	// We might need to clean up tmp/ here as well.
		// 	return fmt.Errorf("failed writing to disk: %w", err)
		// }

		if hash != fileHash {
			panic(fmt.Sprintf("wrong file hash after write, got %s, expected %s", hash, fileHash))
			// return fmt.Errorf("wrong file hash after write, got %s, expected %s", hash, fileHash)
		}

	case PayloadType_RequestFile:
		// A file has been requested by a peer. For now a peer will only request
		// a file if we have notified them that we have it so we don't need to
		// check for the file before streaming it to them.
		data := payload.Data.(PayloadData_RequestFile)
		fileHash := data.FileHash

		f, err := s.store.ReadStream(fileHash)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		peer, ok := s.peers.Get(msg.From)
		if !ok {
			return fmt.Errorf("peer (%s) not found", msg.From)
		}

		// Stream the file data to peer.
		_, err = io.Copy(peer, f)
		if err != nil {
			return fmt.Errorf("failed streaming file to peer (%s): %w", msg.From, err)
		}

		err = f.Close()
		if err != nil {
			return fmt.Errorf("failed to close reader: %w", err)
		}
	}

	return nil
}

// Stores a file and returns the file hash. Gossips the
// file content to other nodes for replicated storage.
func (s *Server) StoreData(r io.Reader) (string, error) {

	// Store file to disk.
	hash, err := s.store.Write(r)
	if err != nil {
		return "", fmt.Errorf("failed to write file to disk: %w", err)
	}

	fileSize, err := s.store.GetFileSize(hash)
	if err != nil {
		return hash, fmt.Errorf("failed to get size of file: %w", err)
	}

	buf := new(bytes.Buffer)
	payload := Payload{
		Type: PayloadType_Notify_NewFile,
		Data: PayloadData_Notify_NewFile{
			FileHash: hash,
			Metadata: Metadata{
				FileSize: fileSize,
			},
		},
	}

	err = gob.NewEncoder(buf).Encode(payload)
	if err != nil {
		return hash, fmt.Errorf("failed to encode file hash to payload data: %w", err)
	}

	msg := &p2p.Message{
		Payload: buf.Bytes(),
	}

	// Notify peers of new file.
	if err := s.BroadcastMessage(msg); err != nil {
		return hash, fmt.Errorf("error broadcasting file to network: %w", err)
	}

	// For each peer we should probably wait for a response so that
	// we know which peers already have the file and which peers need
	// to have the file streamed to them.

	// if err := s.BroadcastData(buf.Bytes()); err != nil {
	// 	return "", fmt.Errorf("error broadcasting file to network: %w", err)
	// }

	return hash, nil
}

func (s *Server) OnPeer(peer p2p.Peer) error {
	s.addPeer(peer)
	return nil
}

func (s *Server) addPeer(peer p2p.Peer) error {
	err := s.peers.Set(peer, peer.RemoteAddr())
	if err != nil {
		return fmt.Errorf("peer already present")
	}
	log.Printf("(%s): Added peer with remote address: %s", s.Transport.LAddr(), peer.RemoteAddr())
	return nil
}

func newSet[T comparable]() *set[T] {
	return &set[T]{
		m:       map[T]struct{}{},
		RWMutex: sync.RWMutex{},
	}
}

type set[T comparable] struct {
	m map[T]struct{}
	sync.RWMutex
}

func (s *set[T]) Add(t T) {
	s.Lock()
	defer s.Unlock()
	s.m[t] = struct{}{}
}

func (s *set[T]) Remove(t T) {
	s.Lock()
	defer s.Unlock()
	delete(s.m, t)
}

func (s *set[T]) Has(t T) bool {
	s.RLock()
	defer s.RUnlock()

	if _, ok := s.m[t]; !ok {
		return false
	}

	return true
}

type syncMap[K comparable, V any] struct {
	m map[K]V
	sync.RWMutex
}

func (s *syncMap[K, V]) Set(k K, v V) {
	s.Lock()
	defer s.Unlock()
	s.m[k] = v
	return
}

func (s *syncMap[K, V]) Get(k K) (V, bool) {
	s.RLock()
	defer s.RUnlock()

	v, ok := s.m[k]
	return v, ok
}

type peerMap struct {
	m map[net.Addr]p2p.Peer
	sync.RWMutex
}

func (p *peerMap) Set(peer p2p.Peer, addr net.Addr) error {
	p.Lock()
	defer p.Unlock()

	if _, ok := p.m[addr]; ok {
		return fmt.Errorf("peer with addr (%s) already present", addr.String())
	}

	p.m[addr] = peer

	return nil
}

func (p *peerMap) Get(addr net.Addr) (peer p2p.Peer, ok bool) {
	p.RLock()
	defer p.RUnlock()

	peer, ok = p.m[addr]
	return peer, ok
}

func (p *peerMap) Values() []p2p.Peer {
	p.RLock()
	defer p.RUnlock()

	values := []p2p.Peer{}
	for _, val := range p.m {
		values = append(values, val)
	}

	return values
}

// // A node will communicate with other nodes to share stored data.
// // It will also expose an API for interacting with the storage system.
// type node struct {
// 	Peers peerMap
// }

// func NewNode() *node {
// 	return &node{}
// }
