package p2p

import "net"

// A message contains arbitrary data that is sent over
// a transport between two nodes.
type Message struct {
	From    net.Addr
	Content []byte
}
