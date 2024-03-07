package p2p

import "net"

// A message contains arbitrary data that is sent over
// a transport between two nodes.
type Message struct {
	// Once we have a handshake defined we could use the
	// persistent public keys to identify the sender instead
	// of a net.Addr.
	From net.Addr

	// Payload will be some encoded bytes with the custom
	// message types as defined and used by the server.
	Payload []byte
}
