package p2p

import (
	"errors"
	"io"
	"log"
	"net"
)

// tcpPeer represents a remote node over a tcp connection.
type tcpPeer struct {
	conn      net.Conn
	isOutboud bool
}

func (t *tcpPeer) Close() error {
	return t.conn.Close()
}

func NewTcpPeer(conn net.Conn, isOutbound bool) *tcpPeer {
	return &tcpPeer{
		conn:      conn,
		isOutboud: isOutbound,
	}
}

type tcpTransport struct {
	TCPTransportOpts
	listener net.Listener
	// peers    peerMap
	msgCh chan Message
}

type TCPTransportOpts struct {
	ListenAddress      string
	ExternalAddress    string
	HandshakeFunc      HandshakeFunc
	OnHandshakeSuccess func(Peer) error
	Decoder            Decoder
}

func NewTcpTransport(opts TCPTransportOpts) *tcpTransport {
	return &tcpTransport{
		TCPTransportOpts: opts,
		// peers:            peerMap{},
		msgCh: make(chan Message),
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
	var err error

	t.listener, err = net.Listen("tcp", t.ListenAddress)
	if err != nil {
		return err
	}

	go func() {
		for {
			conn, err := t.listener.Accept()
			if err != nil {
				log.Printf("Failed to accept TCP connection: %s\n", err)
			}

			log.Printf("New incoming connection %+v\n", conn)
			go t.handleConn(conn)
		}
	}()

	return nil
}

func (t *tcpTransport) MsgChan() <-chan Message {
	return t.msgCh
}

func (t *tcpTransport) handleConn(conn net.Conn) {
	peer := NewTcpPeer(conn, false)

	err := t.HandshakeFunc(peer)
	if err != nil {
		log.Printf("Dropping peer for error: %s\n", err)
		peer.Close()
		return
	}

	err = t.OnHandshakeSuccess(peer)
	if err != nil {
		log.Printf("Dropping peer for error: %s\n", err)
		peer.Close()
	}

	msg := Message{}
	// Read loop
	for {
		err := t.Decoder.Decode(conn, &msg)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				log.Printf("Dropping peer for error: %s", err)
				conn.Close()
				return
			}
			log.Printf("Error decoding message: %s\n", err)
		}

		msg.From = conn.RemoteAddr()

		log.Printf("Recieved message: %+v\n", msg)

		t.msgCh <- msg
	}
}
