package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"

	// "reflect"
	"sync"

	"github.com/edatts/dstore/p2p"
)

func init() {
	// gob.Register(PayloadData_Notify_NewFile{})
	// gob.Register(PayloadData_Request_GetFile{})
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

			if err := s.handleMessage(&msg); err != nil {
				log.Printf("(%s): Failed to process message: %s\n", s.Transport.LAddr(), err)
			}

			p, ok := s.peers.Get(msg.From)
			if !ok {
				log.Println("Could not find peer in peers map.")
				continue
			}

			_ = p

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

	// We shouldn't really use a multiwriter because if we fail
	// to write to one peer then the stream will end to all peers.
	mw := io.MultiWriter(peers...)

	_, err := io.Copy(mw, bytes.NewReader([]byte{p2p.StartStream}))
	if err != nil {
		return fmt.Errorf("failed to copy data to peers: %w", err)
	}

	_, err = io.Copy(mw, r)
	if err != nil {
		return fmt.Errorf("failed to copy data to peers: %w", err)
	}

	if err = r.Close(); err != nil {
		return fmt.Errorf("failed closing read closer: %w", err)
	}

	return nil
}

func (s *Server) BroadcastMessage(m *p2p.Message) {
	for _, peer := range s.peers.Values() {
		if err := peer.Send([]byte{p2p.StartMessage}); err != nil {
			log.Printf("Failed sending message byte to peer (%s): %s", peer.RemoteAddr(), err)
		}
		if err := peer.Send(m.Payload); err != nil {
			log.Printf("Failed sending payload to peer (%s): %s", peer.RemoteAddr(), err)
		}
	}
}

func (s *Server) handleMessage(msg *p2p.Message) error {
	// The issue with handling messages in this way is that a lot of the
	// operations and exchanges between peers that we want to carry out
	// require multiple messages to be sent between peers that need to be
	// processed in a particular order. This function only really works for
	// one off messages because there is no way to prevent race conditions
	// using this central handler.
	//
	// If we need messages to be processed sequentially in a particular
	// order then we need a more robust set of handling logic to ensure
	// that we prevent race conditions.
	//
	// One way we could do this is by multiplexing our connection into
	// multiple channels such that each operation will only receive
	// messages that it would expect to recieve as part of that operation.
	// This would probably be the most performat method but would be
	// complex to implement.
	//
	// Alternatively we could simply queue all the messages we receive
	// for each peer and then loop through them, re-queueing the ones that
	// we wouldn't expect to handle as part of the current operation. This
	// would be less performant and would require timeouts on each operation
	//

	r := bytes.NewReader(msg.Payload)
	payload := Payload{}
	if err := gob.NewDecoder(r).Decode(&payload); err != nil {
		return fmt.Errorf("failed to decode payload: %w", err)
	}

	// log.Printf("Payload Data: %v", payload.Data)
	// log.Printf("Payload Data Type: %v", reflect.TypeOf(payload.Data))

	switch payload.Type {
	case PayloadType_Notify_NewFile:
		peer, ok := s.peers.Get(msg.From)
		if !ok {
			return fmt.Errorf("peer (%s) not found", msg.From)
		}

		// Get file hash and metadata
		data := payload.Data
		// data, err := payload.GetData()
		// if err != nil {
		// 	return fmt.Errorf("invalid payload, could not type assert payload data")
		// }

		fileHash := data.FileHash

		// Check if we have file.
		if s.HasFile(fileHash) {
			log.Printf("(%s): File already present on disk.", s.Transport.LAddr())
			return nil
		}

		// If we are already writing the file, return
		if s.pendingFiles.Has(fileHash) {
			log.Printf("(%s): File already pending.", s.Transport.LAddr())
			return nil
		}

		s.pendingFiles.Add(fileHash)
		defer s.pendingFiles.Remove(fileHash)

		hash, err := s.StoreFile(io.LimitReader(peer, data.Metadata.FileSize))
		if err != nil {
			// We might need to clean up tmp/ here as well.
			return fmt.Errorf("failed to store file: %w", err)
		}

		if hash != fileHash {
			panic(fmt.Sprintf("wrong file hash after write, got %s, expected %s", hash, fileHash))
			// return fmt.Errorf("wrong file hash after write, got %s, expected %s", hash, fileHash)
		}

		peer.WaitGroup().Done()

	case PayloadType_Request_GetFile:
		peer, ok := s.peers.Get(msg.From)
		if !ok {
			return fmt.Errorf("peer (%s) not found", msg.From)
		}

		// A file has been requested by a peer. For now a peer will only request
		// a file if we have notified them that we have it so we don't need to
		// check for the file before streaming it to them.
		data := payload.Data

		// Check for file
		fileHash := data.FileHash

		if !s.HasFile(fileHash) {
			return fmt.Errorf("file (%s) not found", fileHash)
		}

		f, err := s.store.Read(fileHash)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		fileSize, err := s.store.GetFileSize(fileHash)
		if err != nil {
			return fmt.Errorf("failed to get file size: %s", err)
		}

		// Stream the file data to peer.
		err = peer.Send([]byte{p2p.StartStream})
		binary.Write(peer, binary.LittleEndian, fileSize)
		if err != nil {
			return fmt.Errorf("could not start stream with peer")
		}
		_, err = io.Copy(peer, f)
		if err != nil {
			// TODO:  If we fail streaming to peer, drop them.
			return fmt.Errorf("failed streaming file to peer (%s): %w", msg.From, err)
		}

		err = f.Close()
		if err != nil {
			return fmt.Errorf("failed to close reader: %w", err)
		}

	case PayloadType_Query_HasFile:
		// Check if we have file. If we do, then we send the file
		// and the metadata.

		// data, ok := payload.Data.(PayloadData_Query_HasFile)
		// if !ok {
		// 	// TODO: If peer sends us an invalid payload, drop them.
		// 	return fmt.Errorf("invalid payload, could not type assert payload data")
		// }

		// if !s.HasFile(data.FileHash) {

		// 	payload := Payload{
		// 		Type: PayloadType_Response_HasFile,
		// 		Data: PayloadData_Response_HasFile{},
		// 	}

		// }

	}

	return nil
}

func (s *Server) HasFile(fileHash string) bool {
	return s.store.fileExists(fileHash)
}

func (s *Server) GetFile(fileHash string) (io.ReadCloser, error) {
	if s.HasFile(fileHash) {
		return s.store.Read(fileHash)
	}

	log.Printf("(%s): File (%s) not found on disk, requesting file from peers...", s.Transport.LAddr(), fileHash)

	for _, peer := range s.peers.Values() {
		buf := new(bytes.Buffer)
		payload := Payload{
			Type: PayloadType_Request_GetFile,
			Data: PayloadData{
				FileHash: fileHash,
			},
		}

		gob.NewEncoder(buf).Encode(payload)

		peer.Send([]byte{p2p.StartMessage})
		peer.Send(buf.Bytes())

		var fileSize int64
		err := binary.Read(peer, binary.LittleEndian, &fileSize)
		if err != nil {
			return nil, err
		}

		fileHash, err := s.store.Write(io.LimitReader(peer, fileSize))
		if err != nil {
			return nil, err
		}

		log.Printf("(%s): Retrieved and stored file (%s).", s.Transport.LAddr(), fileHash)

		peer.WaitGroup().Done()

	}

	return s.store.Read(fileHash)
}

// Stores a file and returns the file hash. Gossips the
// file content to other nodes for replicated storage.
func (s *Server) StoreFile(r io.Reader) (string, error) {

	// Store file to disk.
	fileHash, err := s.store.Write(r)
	if err != nil {
		return "", fmt.Errorf("failed to write file to disk: %w", err)
	}

	fileSize, err := s.store.GetFileSize(fileHash)
	if err != nil {
		return fileHash, fmt.Errorf("failed to get size of file: %w", err)
	}

	buf := new(bytes.Buffer)
	payload := Payload{
		Type: PayloadType_Notify_NewFile,
		Data: PayloadData{
			FileHash: fileHash,
			Metadata: Metadata{
				FileSize: fileSize,
			},
		},
	}

	err = gob.NewEncoder(buf).Encode(payload)
	if err != nil {
		return fileHash, fmt.Errorf("failed to encode file hash to payload data: %w", err)
	}

	msg := &p2p.Message{
		Payload: buf.Bytes(),
	}

	s.BroadcastMessage(msg)

	f, err := s.store.Read(fileHash)
	if err != nil {
		return fileHash, fmt.Errorf("failed to read file: %w", err)
	}

	err = s.BroadcastData(f)
	if err != nil {
		return fileHash, fmt.Errorf("error broadcasting data: %w", err)
	}

	f.Close()

	return fileHash, nil
}

// Deletes file locally.
func (s *Server) DeleteFile(fileHash string) error {
	if !s.HasFile(fileHash) {
		return nil
	}

	if err := s.store.Delete(fileHash); err != nil {
		return fmt.Errorf("error deleting file: %w", err)
	}

	return nil
}

// Deletes file locally and signals to peers that they
// can delete the file as well.
func (s *Server) PurgeFile(fileHash string) error {

	if err := s.DeleteFile(fileHash); err != nil {
		return fmt.Errorf("failed to delete file (%s): %w", fileHash, err)
	}

	// Notify peers to delete file
	buf := new(bytes.Buffer)
	payload := Payload{
		Type: PayloadType_Request_DeleteFile,
		Data: PayloadData{
			FileHash: fileHash,
		},
	}
	if err := gob.NewEncoder(buf).Encode(payload); err != nil {
		return fmt.Errorf("failed to encode deleteFile payload: %w", err)
	}

	msg := p2p.Message{
		Payload: buf.Bytes(),
	}

	s.BroadcastMessage(&msg)

	// We could mark the file as purge pending and we could
	// periodically check to see if the purge is complete. This
	// would need to be done asynchronously.

	return nil
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
