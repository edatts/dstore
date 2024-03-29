package p2p

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTCPTransport(t *testing.T) {
	listenAddr := ":3010"
	externalAddr := "127.0.0.1:3010"
	opts := TCPTransportOpts{
		ListenAddress:   listenAddr,
		ExternalAddress: externalAddr,
		HandshakeFunc:   DefaultHandshake,
		Decoder:         DefaultDecoder{},
	}
	transport := NewTCPTransport(opts)

	assert.Equal(t, transport.ListenAddress, listenAddr)

	assert.Nil(t, transport.ListenAndAccept())

}
