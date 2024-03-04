package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	// "net"
	// "sync"

	"github.com/edatts/dstore/p2p"
)

type ServerOpts struct {
	StorageRoot    string
	Transport      p2p.Transport
	BootstrapNodes []string
}

type Server struct {
	ServerOpts

	peers  peerMap
	store  *Store
	quitCh chan struct{}
}

func NewServer(opts ServerOpts) *Server {
	storeOpts := StoreOpts{
		StorageRoot: opts.StorageRoot,
	}

	return &Server{
		ServerOpts: opts,

		peers: peerMap{
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
	defer func() {
		log.Println("Message received on quit channel, stopping server.")
		s.Transport.Close()
	}()

	for {
		select {
		case msg := <-s.Transport.MsgChan():
			log.Printf("Received message: %v", msg)
		case <-s.quitCh:
			return
		}
	}
}

// Stores a file and returns the file hash.
func (s *Server) StoreFile(r io.Reader) (string, error) {
	return s.store.Write(r)
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
	log.Printf("Added peer with remote address: %s", peer.RemoteAddr())
	return nil
}

type peerMap struct {
	m map[net.Addr]p2p.Peer
	sync.RWMutex
}

func (p *peerMap) Set(peer p2p.Peer, addr net.Addr) error {
	p.Lock()
	defer p.Unlock()

	if _, ok := p.m[addr]; ok {
		return fmt.Errorf("peer with addr (%s) already present.", addr.String())
	}

	p.m[addr] = peer

	return nil
}

func (p *peerMap) Get(addr net.Addr) (peer p2p.Peer, ok bool) {
	p.Lock()
	defer p.Unlock()

	peer, ok = p.m[addr]
	return peer, ok
}

// // A node will communicate with other nodes to share stored data.
// // It will also expose an API for interacting with the storage system.
// type node struct {
// 	Peers peerMap
// }

// func NewNode() *node {
// 	return &node{}
// }
