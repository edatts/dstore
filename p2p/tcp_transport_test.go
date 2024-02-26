package p2p

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTcpTransport(t *testing.T) {
	listenAddr := ":3010"
	opts := TCPTransportOpts{
		ListenAddress: listenAddr,
		HandshakeFunc: DefaultHandshake,
		Decoder:       GobDecoder{},
	}
	transport := NewTcpTransport(opts)

	assert.Equal(t, transport.ListenAddress, listenAddr)

	assert.Nil(t, transport.ListenAndAccept())

}
