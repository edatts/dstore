package p2p

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

// tcpPeer represents a remote node over a tcp connection.
type TCPPeer struct {
	// Embedding the underlying connection for the peer allows
	// us to make use of io.Reader and io.Writer directly on
	// the peer.
	net.Conn

	isOutboud bool

	wg *sync.WaitGroup
}

func NewTcpPeer(conn net.Conn, isOutbound bool) *TCPPeer {
	return &TCPPeer{
		Conn:      conn,
		isOutboud: isOutbound,
		wg:        &sync.WaitGroup{},
	}
}

func (t *TCPPeer) Send(b []byte) error {
	n, err := t.Conn.Write(b)
	if err != nil {
		return fmt.Errorf("error sending to peer (%s): %w", t.RemoteAddr(), err)
	}

	log.Printf("Wrote %d bytes.", n)

	return nil
}

func (t *TCPPeer) WaitGroup() *sync.WaitGroup {
	return t.wg
}

type TCPTransport struct {
	TCPTransportOpts
	listener net.Listener
	// peers    peerMap
	msgCh chan Message
}

type TCPTransportOpts struct {
	ListenAddress   string
	ExternalAddress string
	HandshakeFunc   HandshakeFunc
	OnPeer          func(Peer) error
	Decoder         Decoder
}

func NewTCPTransport(opts TCPTransportOpts) *TCPTransport {
	return &TCPTransport{
		TCPTransportOpts: opts,
		// peers:            peerMap{},
		msgCh: make(chan Message),
	}
}

func (t *TCPTransport) Dial(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}

	go t.handleConn(conn, true)

	return nil
}

func (t *TCPTransport) ListenAndAccept() error {
	var err error

	t.listener, err = net.Listen("tcp", t.ListenAddress)
	if err != nil {
		return err
	}

	log.Printf("Starting listener on port (%s)\n", t.ListenAddress)

	go func() {
		for {
			conn, err := t.listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				log.Printf("Failed to accept TCP connection: %s\n", err)
				continue
			}

			log.Printf("(%s): New incoming connection %+v\n", t.LAddr(), conn)
			go t.handleConn(conn, false)
		}
	}()

	return nil
}

func (t *TCPTransport) Close() error {
	if err := t.listener.Close(); err != nil {
		return fmt.Errorf("error closing listener: %w", err)
	}
	return nil
}

func (t *TCPTransport) MsgChan() <-chan Message {
	return t.msgCh
}

func (t *TCPTransport) handleConn(conn net.Conn, isOutbound bool) {
	peer := NewTcpPeer(conn, isOutbound)

	err := t.HandshakeFunc(peer)
	if err != nil {
		log.Printf("Handshake failed: %s\n", err)
		// log.Printf("Dropping peer for error: %s\n", err)
		peer.Close()
		return
	}

	err = t.OnPeer(peer)
	if err != nil {
		log.Printf("Dropping peer for error: %s\n", err)
		peer.Close()
		return
	}

	msg := Message{}
	// Read loop
	for {
		buf := make([]byte, 2024)
		n, err := conn.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				log.Printf("Dropping peer for error: %s", err)
				conn.Close()
				return
			}
			log.Printf("Error reading message: %s\n", err)
			// Should we drop peer after n message errors?
		}

		msg.From = conn.RemoteAddr()
		msg.Payload = buf[:n]

		// log.Printf("Recieved message from: %s\n", conn.RemoteAddr().String())
		// log.Printf("Message content: %s\n", string(msg.Content))

		peer.wg.Add(1)
		t.msgCh <- msg
		// log.Println("Pausing read loop, waiting for message to be processed.")
		peer.wg.Wait()
		log.Printf("(%s): Message processed, resuming read loop.", t.LAddr())
	}
}

func (t *TCPTransport) LAddr() string {
	return t.ListenAddress
}
