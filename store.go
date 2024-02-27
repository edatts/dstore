package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"os"
)

func DefaultPathTransformFunc(key string) string {
	return key + "/"
}

// Transforms the key into a path consisting of hash parts.
// The path has a trailing "/" so the filename can be appended
// directly onto the end of the path string.
func CASPathTransformFunc(key string) string {
	hashBytes := sha256.Sum256([]byte(key))
	hash := hex.EncodeToString(hashBytes[:])

	blockSize := 4
	path := ""
	for i := 0; i < blockSize; i++ {
		path = path + hash[i*blockSize:(i*blockSize)+blockSize] + "/"
	}

	return path
}

type StoreOpts struct {
	PathTransformFunc func(string) string
}

type Store struct {
	StoreOpts
}

func NewStore(opts StoreOpts) *Store {

	return &Store{
		opts,
	}
}

func (s *Store) WriteStream(key string, r io.Reader) error {

	path := s.PathTransformFunc(key)

	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return err
	}

	filename := "testFileName"
	filePath := path + filename

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}

	n, err := io.Copy(f, r)
	if err != nil {
		return err
	}

	log.Printf("Wrote %v bytes to disk at location: %s", n, filePath)

	return nil
}
