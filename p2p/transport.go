package p2p

import "net"

// Peer is any legitimate node on the network that we can connect to.
type Peer interface {
	Send([]byte) error
	RemoteAddr() net.Addr
	Close() error
}

// Transport will be a method for nodes to communicate with eachother.
type Transport interface {
	Dial(string) error
	ListenAndAccept() error
	MsgChan() <-chan Message
	Close() error
}
