package p2p

// import "net"

type Handshaker interface {
	Handshake() error
}

type HandshakeFunc func(any) error

func defaultHandshake(any) error { return nil }
