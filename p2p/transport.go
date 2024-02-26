package p2p

import ()

// Peer is any legitimate node on the network that we can connect to.
type Peer interface {
	Close() error
}

// Transport will be a method for nodes to communicate with eachother.
type Transport interface {
	ListenAndAccept() error
	MsgChan() <-chan Message
}
