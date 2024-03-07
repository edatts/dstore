package p2p

import (
	"net"
	"sync"
)

// Peer is any legitimate node on the network that we can connect to.
type Peer interface {
	net.Conn
	Send([]byte) error
	WaitGroup() *sync.WaitGroup
}

// Transport will be a method for nodes to communicate with eachother.
type Transport interface {
	Dial(string) error
	ListenAndAccept() error
	MsgChan() <-chan Message
	Close() error
	LAddr() string
}
