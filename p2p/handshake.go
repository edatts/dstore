package p2p

// import "net"

// A handshake is used to verify authenticity of the connection.
type HandshakeFunc func(any) error

func defaultHandshake(any) error { return nil }
