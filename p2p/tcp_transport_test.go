package p2p

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTcpTransport(t *testing.T) {
	listenAddr := ":3010"
	transport := NewTcpTransport(listenAddr)

	assert.Equal(t, transport.listenAddress, listenAddr)

	assert.Nil(t, transport.ListenAndAccept())

}
