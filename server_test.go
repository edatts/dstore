package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/edatts/dstore/p2p"
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

// type SmallFiles struct {
// 	Hashes  []string
// 	Content []string
// }

func getRandomString(numChars int) string {
	letters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	strBytes := make([]byte, numChars)

	for i := 0; i < numChars; i++ {
		strBytes[i] = letters[rand.Intn(len(letters))]
	}

	return string(strBytes)
}

// Let's make some small test files
// var smallFiles = SmallFiles{}
// for i := 0; i < 100; i++ {
// 	content := getRandomString(50)
// 	hashBytes := sha256.Sum256([]byte(content))
// 	hash := hex.EncodeToString(hashBytes[:])

// 	smallFiles.Hashes = append(smallFiles.Hashes, hash)
// 	smallFiles.Content = append(smallFiles.Content, content)
// }

// f, err := os.Create("test-files/small-files.json")
// if err != nil {
// 	log.Fatal(err)
// }

// json.NewEncoder(f).Encode(smallFiles)
// os.Exit(0)

// Let's make some big test files
// f, err := os.Create("test-files/500MiB")
// if err != nil {
// 	log.Fatal(err)
// }

// n, err := io.CopyN(f, rand.Reader, 500*1024*1024)
// if err != nil {
// 	log.Fatal(err)
// }

// log.Printf("Wrote %d KiB", n/1024)
// os.Exit(0)

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
	_, err := s1.StoreFile(data, true)
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
		t.Errorf("wrong file content, expected (%s) got (%s)", fileContent, string(gotFileBytes))
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

	// We also want to test to make sure that nodes will still
	// respond to requests while they are streaming large files.

	// TODO: Modify to make use of testing package and add
	// 		 assertations and failure cases.
	// Store large file in s1.
	f, err := os.Open("test-files/2GiB")
	if err != nil {
		log.Fatalf("failed to open 2GiB file: %s", err)
	}

	startTime := time.Now().UnixMilli()
	hash, err := s1.store.Write(f)
	if err != nil {
		log.Fatalf("failed to write file to store: %s", err)
	}

	f.Close()

	log.Printf("Saved 2GiB file with hash (%s)", hash)
	log.Printf("Time consumed: %d ms", time.Now().UnixMilli()-startTime)

	// Store some small files in b1 so we can run requests on them
	f, err = os.Open("test-files/small-files.json")
	if err != nil {
		log.Fatal(err)
	}

	smallFiles := SmallFiles{}
	json.NewDecoder(f).Decode(&smallFiles)

	for i := 0; i < len(smallFiles.Hashes); i++ {
		r := bytes.NewReader([]byte(smallFiles.Content[i]))
		hash, err := b1.store.Write(r)
		if err != nil {
			log.Fatal(err)
		}

		if hash != smallFiles.Hashes[i] {
			log.Fatalf("different hash after storage, got (%s), expected (%s)", hash, smallFiles.Hashes[i])
		}
	}

	// Now stream large file from s1 to b1 while making concurrent
	// requests from s1 to b1 but also from b1 to s1.
	fileSize, err := s1.store.GetFileSize(hash)
	if err != nil {
		log.Fatalf("failed getting file size: %s", err)
	}

	for _, peer := range s1.peers.Values() {
		if peer.RemoteAddr().String() == "127.0.0.1:3010" {
			channel, err := peer.OpenChannel()
			if err != nil {
				log.Fatalf("failed opening channel: %s", err)
			}

			res, err := s1.MakePutFileRequest(hash, int(fileSize), peer, channel.Id(), false)
			if err != nil {
				log.Fatal(err)
			}

			if res.Result {
				startTime := time.Now().UnixMilli()

				// Start large file stream
				go func() {
					log.Printf("Started streaming large file.")

					f, err := s1.store.Read(hash)
					if err != nil {
						log.Fatal(err)
					}

					defer f.Close()

					n, err := io.Copy(channel, f)
					if err != nil {
						log.Fatal(err)
					}

					log.Printf("Wrote %d MiB to peer in %d ms", n/1024/1024, time.Now().UnixMilli()-startTime)
				}()

				// Start requests
				channels := getNChannels(5, peer) // s1 -> b1 channels
				go func(peer p2p.Peer) {
					time.Sleep(time.Second)
					// Make a hasFile request on every file across all channels.
					for i := 0; i < len(smallFiles.Hashes); i++ {
						res, err := s1.MakeHasFileRequest(smallFiles.Hashes[i], peer, channels[i%len(channels)].Id())
						if err != nil {
							log.Fatal(err)
						}

						if !res.Result {
							log.Fatal("Expected only true results for hasFile requests...")
						}

						// Make getFile request on every 5th file
						if i%5 == 4 {
							if err := s1.GetFileFromPeer(smallFiles.Hashes[i], peer); err != nil {
								log.Fatal(err)
							}

							log.Printf("Time elapsed: %d ms", time.Now().UnixMilli()-startTime)
						}
					}

				}(peer)

			}
		}
	}

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
