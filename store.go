package main

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
)

func DefaultPathTransformFunc(key string) string {
	return key + "/"
}

// Transforms the key into a path consisting of hash parts.
// The path has a trailing "/" so the filename can be appended
// directly onto the end of the path string.
func CASPathTransformFunc(key string) string {
	hashBytes := md5.Sum([]byte(key))
	hash := hex.EncodeToString(hashBytes[:])
	// log.Printf("Hash: %v, len(hash): %v", hash, len(hash))

	blockSize := 4
	pathLen := len(hash) / blockSize
	path := ""
	for i := 0; i < pathLen; i++ {
		path = path + hash[i*blockSize:(i*blockSize)+blockSize] + "/"
	}

	return path
}

// Creates the file path from the data streamed through the
// input reader, this way the path and file name are entirely
// derived from the content of the file to be stored.
func PathTransformFunc(r io.Reader) (string, string, error) {
	sha256Hash := sha256.New()
	_, err := io.Copy(sha256Hash, r)
	if err != nil {
		return "", "", fmt.Errorf("failed to copy data from reader: %w", err)
	}
	hashBytes := sha256Hash.Sum(nil)
	hexHash := hex.EncodeToString(hashBytes)

	log.Printf("len(hashBytes): %v", len(hashBytes))
	log.Printf("len(hexHash): %v", len(hexHash))

	blockSize := 8
	pathLen := len(hexHash) / blockSize
	path := ""
	for i := 0; i < pathLen/2; i++ {
		path = path + hexHash[i*blockSize:(i*blockSize)+blockSize] + "/"
	}
	fileName := hexHash[pathLen*blockSize/2:]

	log.Printf("Path: %v\n", path)
	log.Printf("File name: %v\n", fileName)

	return path, fileName, nil
}

type StoreOpts struct {
	// PathTransformFunc func(string) string
	PathTransformFunc func(io.Reader) (string, string, error)
}

type Store struct {
	StoreOpts
}

func NewStore(opts StoreOpts) *Store {
	return &Store{
		opts,
	}
}

func (s *Store) ReadStream(key string) (io.Reader, error) {
	pathKey := CASPathTransformFunc(key)

	_ = pathKey

	// f, err := os.Open(pathKey)

	return nil, nil

}

func (s *Store) WriteStream(r io.Reader) error {
	// We'll need to write the data stream to a temp file and then
	// generate the appropriate path from the file content before
	// commiting the file.

	b := make([]byte, 16)
	rand.Read(b)
	err := os.MkdirAll("tmp", os.ModePerm)
	if err != nil {
		return err
	}
	tempFilePath := fmt.Sprintf("tmp/%s", hex.EncodeToString(b))
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		return err
	}

	tr := io.TeeReader(r, tempFile)

	path, fileName, err := s.PathTransformFunc(tr)
	if err != nil {
		return err
	}

	log.Printf("File path: %v", path+fileName)

	err = os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return err
	}

	cmd, err := exec.Command("ls", "-al").Output()
	if err != nil {
		log.Printf("Error: %v", err)
	}

	log.Printf("cmd: %s", cmd)

	cmd, err = exec.Command("mv", tempFilePath, path+fileName).Output()
	if err != nil {
		log.Printf("Error: %v", err)
	}
	log.Printf("cmd: %v", cmd)

	return nil
}

// func (s *Store) WriteStream(key string, r io.Reader) error {

// 	path := s.PathTransformFunc(key)

// 	err := os.MkdirAll(path, os.ModePerm)
// 	if err != nil {
// 		return err
// 	}

// 	// fileName will be a hash of the content of the file
// 	md5Hash := md5.New()
// 	io.Copy(md5Hash, r)
// 	hashBytes := md5Hash.Sum(nil)
// 	fileName := hex.EncodeToString(hashBytes)
// 	filePath := path + fileName

// 	f, err := os.Create(filePath)
// 	if err != nil {
// 		return err
// 	}

// 	n, err := io.Copy(f, r)
// 	if err != nil {
// 		return err
// 	}

// 	log.Printf("Wrote %v bytes to disk at location: %s", n, filePath)

// 	return nil
// }
