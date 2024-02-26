package main

import (
	"log"

	"github.com/edatts/dstore/p2p"
)

func init() {
	// Maybe do some stuff here

}

func main() {
	// Spin up two nodes locally
	// node1 := NewTcpNode()
	// node2 := NewTcpNode()

	handshakeSuccessFunc := func(peer p2p.Peer) error {
		// Add peer to server.
		peer.Close()

		return nil
	}

	tcpOpts := p2p.TCPTransportOpts{
		ListenAddress:      ":3010",
		ExternalAddress:    "127.0.0.1:3010",
		HandshakeFunc:      p2p.DefaultHandshake,
		OnHandshakeSuccess: handshakeSuccessFunc,
		// Decoder:       p2p.GobDecoder{},
		Decoder: p2p.DefaultDecoder{},
	}
	tr := p2p.NewTcpTransport(tcpOpts)

	tcpOpts.ListenAddress = ":3011"
	tcpOpts.ExternalAddress = "127.0.0.1:3011"
	tr2 := p2p.NewTcpTransport(tcpOpts)

	go func() {
		for {
			msg := <-tr.MsgChan()

			log.Printf("handling message: %v\n", msg.Content)
		}
	}()

	err := tr.ListenAndAccept()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		conn, err := tr2.Dial(tr.ExternalAddress)
		if err != nil {
			log.Printf("%v\n", tr.ExternalAddress)
			log.Fatal(err)
		}

		conn.Write([]byte("Hello there"))
		conn.Close()
	}()

	select {}
}
