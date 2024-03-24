package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"testing"
	"time"
	// "github.com/edatts/dstore/p2p"
)

// func makeServer(lAddr string, bootstrapNodes []string) *Server {
// 	tcpOpts := p2p.TCPTransportOpts{
// 		ListenAddress:   lAddr,
// 		ExternalAddress: "127.0.0.1" + lAddr,
// 		HandshakeFunc:   p2p.DefaultHandshake,
// 		Decoder:         p2p.DefaultDecoder{},
// 		// Decoder:       p2p.GobDecoder{},
// 	}
// 	tr := p2p.NewTCPTransport(tcpOpts)

// 	serverOpts := ServerOpts{
// 		StorageRoot:    "test-storage/local" + lAddr,
// 		Transport:      tr,
// 		BootstrapNodes: bootstrapNodes,
// 	}

// 	s := NewServer(serverOpts)
// 	s.Transport.(*p2p.TCPTransport).OnPeer = s.OnPeer

// 	return s
// }

func makeNetwork(numNodes int) []*Server {

	return nil
}

func TestServer(t *testing.T) {

	b1 := makeServer(":3010", []string{})
	b2 := makeServer(":3011", []string{})

	bNodes := []string{":3010", ":3011"}

	s1 := makeServer(":3012", bNodes)
	s2 := makeServer(":3013", bNodes)
	s3 := makeServer(":3014", bNodes)

	servers := []*Server{b1, b2, s1, s2, s3}

	for _, s := range servers {
		go func(s *Server) {
			log.Fatal(s.Start())
		}(s)
	}

	// Test storing file and ensure peers have it.
	fileContent := "I am the content of a file."
	fileBytes := []byte(fileContent)
	fileHashBytes := sha256.Sum256(fileBytes)
	fileHash := hex.EncodeToString(fileHashBytes[:])
	_ = fileHash

	data := bytes.NewReader(fileBytes)
	_, err := s1.StoreFile(data)
	if err != nil {
		t.Errorf("failed storing file: %s", err)
	}

	time.Sleep(time.Second * 3)

	for _, s := range servers {
		if !s.HasFile(fileHash) {
			t.Errorf("Peer (%s) does not have file (%s)", s.Transport.LAddr(), fileHash)
		}
	}

	// Test deleting file that we have. Ensure the file is removed
	// only from the local store.

	if err := s1.DeleteFile(fileHash); err != nil {
		t.Errorf("failed deleting file (%s): %s", fileHash, err)
	}

	if s1.HasFile(fileHash) {
		t.Errorf("server has file (%s), expected no file", fileHash)
	}

	for _, s := range servers {
		if s.Transport.LAddr() == s1.Transport.LAddr() {
			continue
		}

		if !s.HasFile(fileHash) {
			t.Errorf("server (%s) does not have file (%s), expected file.", s.Transport.LAddr(), fileHash)
		}
	}

	// Test getting a file we don't have locally but the network
	// does have. Ensure the file received is the same.

	r, err := s1.GetFile(fileHash)
	if err != nil {
		t.Errorf("failed getting file: %s", err)
	}

	gotFileBytes, err := io.ReadAll(r)
	if err != nil {
		log.Fatalf("failed to read all gotFileBytes: %s", err)
	}

	if fileContent != string(gotFileBytes) {
		t.Error("wrong file content, expected (%s) got (%s)", fileContent, string(gotFileBytes))
	}

	// Test deleting file on peer. Ensure file is only deleted on
	// that peer.

	// Test purging file from network. Ensure entire network removes
	// file.

	if err := s1.PurgeFile(fileHash); err != nil {
		t.Errorf("failed purging file (%s): %s", fileHash, err)
	}

	for _, s := range servers {
		if s.HasFile(fileHash) {
			t.Errorf("file (%s) was not purged from server (%s)", fileHash, s.Transport.LAddr())
		}
	}

	// Test purging file from network while file is already not
	// present locally.

}

// func TestNetworkHasFile() {

// }

// func TestNetworkPutFile() {

// }

// func TestNetworkGetFile() {

// }

// func TestNetworkDeleteFile() {

// }

// func TestNetworkGetDiskSpace() {

// }

// func TestSendingRequestsDuringFileStream() {

// }

// func TestReceivingRequestsDuringFileStream() {

// }
