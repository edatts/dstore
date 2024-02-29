package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const (
	blockSize int = 8
)

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

	pathLen := len(hexHash) / blockSize
	path := ""
	for i := 0; i < pathLen/2; i++ {
		path = path + hexHash[i*blockSize:(i*blockSize)+blockSize] + "/"
	}
	CASFileName := hexHash[pathLen*blockSize/2:]

	// log.Printf("Path: %v\n", path)
	// log.Printf("File name: %v\n", fileName)

	return path, CASFileName, nil
}

// Validates the provided hash then trnasforms it into the
// corresponding file path.
func FileHashToFilePathFunc(fileHash string) (string, error) {

	if len(fileHash) != 64 {
		return "", fmt.Errorf("invalid file hash: wrong length")
	}

	pathLen := len(fileHash) / blockSize
	path := ""
	for i := 0; i < pathLen/2; i++ {
		path = path + fileHash[i*blockSize:(i*blockSize)+blockSize] + "/"
	}
	CASFileName := fileHash[pathLen*blockSize/2:]

	return path + CASFileName, nil
}

type StoreOpts struct {
	PathTransformFunc      func(io.Reader) (string, string, error)
	FileHashToFilePathFunc func(string) (string, error)
}

type Store struct {
	StoreOpts
}

func NewStore(opts StoreOpts) *Store {
	return &Store{
		opts,
	}
}

type BufferedReader struct {
	buf []byte
}

// Read() starts the stream reader, returns an error
// if the file cannot be found.
func (s *BufferedReader) Read(fileHash string) error {

	return nil
}

// Next() returns true if there is more data and false
// otherwise. The first call of Next() prepares the
// first chunk of data.
func (s *BufferedReader) Next() bool {

	return false
}

// Data() returns the current chunk of buffered data.
// Next() should be called before calling Data(), calling
// Data() before Next() will return an empty byte slice.
func (s *BufferedReader) Data() []byte {

	return s.buf
}

// ReadBuffered() returns a pointer to a BufferedReader,
// which can be used to fetch the stored file in chunks.
// An error is returned if the requested file cannot be
// found.
func (s *Store) ReadBuffered(fileHash string) (*BufferedReader, error) {

	return nil, nil
}

// Read will read the file into a byte slice and return it.
// If the requested file is too big then Read() will return
// an error. An error will also be returned if the requested
// file cannot be found.
func (s *Store) Read(fileHash string) ([]byte, error) {

	return nil, nil
}

// Readstream takes the file hash, uses it to find the corresponding
// file path and returns the file handle to be read from.
func (s *Store) ReadStream(fileHash string) (io.ReadCloser, error) {

	filePath, err := FileHashToFilePathFunc(fileHash)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	return f, nil
}

// The data stream is written to a temp file and then the
// corresponding path is generated from the file content before
// the temp files is moved to it's final location.
// Returns the hash of the original file.
func (s *Store) WriteStream(r io.Reader) (string, error) {

	b := make([]byte, 16)
	rand.Read(b)
	err := os.MkdirAll("tmp", os.ModePerm)
	if err != nil {
		return "", err
	}
	tempFilePath := fmt.Sprintf("tmp/%s", hex.EncodeToString(b))
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		return "", err
	}

	tr := io.TeeReader(r, tempFile)

	path, CASFileName, err := s.PathTransformFunc(tr)
	if err != nil {
		return "", err
	}

	fileHash := strings.Join(strings.Split(path, "/"), "") + CASFileName

	// log.Printf("File path: %v", path+fileName)

	err = os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return "", err
	}

	_, err = exec.Command("ls", "-al").Output()
	if err != nil {
		return "", err
	}

	_, err = exec.Command("mv", tempFilePath, path+CASFileName).Output()
	if err != nil {
		return "", err
	}

	return fileHash, nil
}
