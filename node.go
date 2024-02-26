package main

import (
	"net"
	"sync"

	"github.com/edatts/dstore/p2p"
)

// Embedded mutex so we don't lock the whole transport struct.
type peerMap struct {
	m  map[net.Addr]p2p.Peer
	mu sync.RWMutex
}

func (p *peerMap) Set(peer p2p.Peer, addr net.Addr) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.m[addr] = peer
}

func (p *peerMap) Get(addr net.Addr) (peer p2p.Peer, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	peer, ok = p.m[addr]
	return peer, ok
}

// A node will communicate with other nodes to share stored data.
// It will also expose an API for interacting with the storage system.
type node struct {
	Peers peerMap
}

func NewNode() *node {
	return &node{}
}
