package p2p

import (
	"log"
	"net"
	"sync"
)

// Embedded mutex so we don't lock the whole transport struct.
type peerMap struct {
	m  map[net.Addr]Peer
	mu sync.RWMutex
}

func (p *peerMap) Set(peer Peer, addr net.Addr) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.m[addr] = peer
}

func (p *peerMap) Get(addr net.Addr) (peer Peer, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	peer, ok = p.m[addr]
	return peer, ok
}

// tcpPeer represents a remote node over a tcp connection.
type tcpPeer struct {
	conn      net.Conn
	isOutboud bool
}

func NewTcpPeer(conn net.Conn, isOutbound bool) *tcpPeer {
	return &tcpPeer{
		conn:      conn,
		isOutboud: isOutbound,
	}
}

type tcpTransport struct {
	listenAddress string
	// listener      net.Listener

	peers peerMap

	startHandshake HandshakeFunc
}

func NewTcpTransport(lAddr string) *tcpTransport {
	return &tcpTransport{
		listenAddress:  lAddr,
		peers:          peerMap{},
		startHandshake: defaultHandshake,
	}
}

func (t *tcpTransport) Dial(addr string) (net.Conn, error) {

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (t *tcpTransport) ListenAndAccept() error {

	listener, err := net.Listen("tcp", t.listenAddress)
	if err != nil {
		return err
	}

	// t.listener = listener

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Failed to accept TCP connection: %s\n", err)
			}

			go t.handleConn(conn)
		}
	}()

	return nil
}

func (t *tcpTransport) handleConn(conn net.Conn) {
	log.Printf("New incoming connection %+v\n", conn)

	peer := NewTcpPeer(conn, false)

	err := t.startHandshake(conn)
	if err != nil {
		log.Printf("Handshake failed...")
		return
	}

	_ = peer
}
