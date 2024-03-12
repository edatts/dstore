package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"

	// "time"

	"github.com/edatts/dstore/p2p"
)

func init() {
	// Maybe do some stuff here

}

func makeServer(lAddr string, bootstrapNodes []string) *Server {
	tcpOpts := p2p.TCPTransportOpts{
		ListenAddress:   lAddr,
		ExternalAddress: "127.0.0.1" + lAddr,
		HandshakeFunc:   p2p.DefaultHandshake,
		Decoder:         p2p.DefaultDecoder{},
		// Decoder:       p2p.GobDecoder{},
	}
	tr := p2p.NewTCPTransport(tcpOpts)

	serverOpts := ServerOpts{
		StorageRoot:    "test-storage/local" + lAddr,
		Transport:      tr,
		BootstrapNodes: bootstrapNodes,
	}

	s := NewServer(serverOpts)
	s.Transport.(*p2p.TCPTransport).OnPeer = s.OnPeer

	return s
}

func main() {

	b1 := makeServer(":3010", []string{})
	b2 := makeServer(":3011", []string{})

	bNodes := []string{":3010", ":3011"}

	s1 := makeServer(":3012", bNodes)
	s2 := makeServer(":3013", bNodes)
	s3 := makeServer(":3014", bNodes)

	for _, s := range []*Server{b1, b2, s1, s2, s3} {
		go func(s *Server) {
			log.Fatal(s.Start())
		}(s)
	}

	fileBytes := []byte("I am the content of a file.")
	fileHashBytes := sha256.Sum256(fileBytes)
	fileHash := hex.EncodeToString(fileHashBytes[:])
	_ = fileHash

	data := bytes.NewReader(fileBytes)
	_, err := s1.StoreFile(data)
	if err != nil {
		log.Fatal(err)
	}

	_, err = s2.GetFile(fileHash)
	if err != nil {
		log.Fatal("Could not get file")
	}

	noFileHashBytes := sha256.Sum256([]byte("File we don't have"))
	noFileHash := hex.EncodeToString(noFileHashBytes[:])
	_ = noFileHash

	r, err := s2.GetFile(noFileHash)
	if err != nil {
		log.Fatal("Could not get file")
	}

	gotFileBytes, err := io.ReadAll(r)
	if err != nil {
		log.Fatal("failed to read all gotFileBytes.")
	}

	log.Printf("Got file with content: %s", string(gotFileBytes))

	select {}

	////

	// tcpOpts := p2p.TCPTransportOpts{
	// 	ListenAddress:      ":3010",
	// 	ExternalAddress:    "127.0.0.1:3010",
	// 	HandshakeFunc:      p2p.DefaultHandshake,
	// 	OnHandshakeSuccess: p2p.DefaultOnHandshakeSuccess,
	// 	Decoder:            p2p.DefaultDecoder{},
	// 	// Decoder:       p2p.GobDecoder{},
	// }
	// tr := p2p.NewTCPTransport(tcpOpts)

	// serverOpts := ServerOpts{
	// 	StorageRoot:    "local_3010",
	// 	Transport:      tr,
	// 	BootstrapNodes: []string{":4010"},
	// }

	// server := NewServer(serverOpts)

	// go func() {
	// 	time.Sleep(time.Second * 10)
	// 	server.Stop()
	// }()

	// if err := server.Start(); err != nil {
	// 	log.Fatal(err)
	// }

	// go func() {
	// 	for {
	// 		msg := <-tr.MsgChan()

	// 		log.Printf("handling message: %v\n", msg.Content)
	// 	}
	// }()

	// select {}
}
