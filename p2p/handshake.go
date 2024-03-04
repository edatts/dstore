package p2p

import "errors"

// import "net"

// ErrFailedHandshake is returned if the handshake between
// the local and remote node was not successful.
var ErrFailedHandshake = errors.New("handshake failed")

// A handshake is used to verify authenticity of the connection.
type HandshakeFunc func(any) error

func DefaultHandshake(any) error {

	var ok = true
	if !ok {
		return ErrFailedHandshake
	}
	return nil
}

func DefaultOnHandshakeSuccess(p Peer) error {

	return nil
}
