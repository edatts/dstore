package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

const (
	blockSize        int = 8
	maxFileReadBytes int = 200 * 1024 * 1024 // 200 MiB
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

	if !isValidHex(fileHash) {
		return "", fmt.Errorf("invalid file hash: input is not valid hex")
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

// Start() starts the stream reader, returns an error
// if the file cannot be found.
func (s *BufferedReader) Start() error {

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

	// Get file path
	fullPath, err := FileHashToFilePathFunc(fileHash)
	if err != nil {
		return nil, fmt.Errorf("failed to convert hash to file path: %w", err)
	}

	// Validate that file exists on disk
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %w", err)
		}
		return nil, err
	}

	// Validate size of file
	if fileInfo.Size() > int64(maxFileReadBytes) {
		return nil, fmt.Errorf("file too big, use the provided buffered read method")
	}

	r, err := s.readStream(fileHash)
	if err != nil {
		return nil, err
	}

	defer r.Close()

	fileBytes := make([]byte, fileInfo.Size())
	_, err = r.Read(fileBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read file into buffer: %w", err)
	}

	return fileBytes, nil
}

// The readStream method takes the file hash, uses it to
// find the corresponding file path and returns the file
// handle to be read from.
func (s *Store) readStream(fileHash string) (io.ReadCloser, error) {

	filePath, err := FileHashToFilePathFunc(fileHash)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %w", err)
	}

	return f, nil
}

func (s *Store) Write(r io.Reader) (string, error) {
	fileHash := ""

	fileHash, err := s.writeStream(r)
	if err != nil {
		return "", err
	}

	return fileHash, nil
}

// The data stream is written to a temp file and then the
// corresponding path is generated from the file content before
// the temp files is moved to it's final location.
// Returns the hash of the original file.
func (s *Store) writeStream(r io.Reader) (string, error) {

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

// The Delete() method removes a file from the storage if it is
// found. Calling Delete() with a file hash that has no
// corresponding file will return an error. Providing an invalid
// file hash will return an error.
func (s *Store) Delete(fileHash string) error {

	// Get file path
	fullPath, err := FileHashToFilePathFunc(fileHash)
	if err != nil {
		return fmt.Errorf("failed to convert hash to file path: %w", err)
	}

	// Check if file exists.
	_, err = os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no such file to delete: %w", err)
		}
	}

	// Delete file and any empty directories
	paths := getDeletePaths(fullPath)

	for _, path := range paths {
		err = os.Remove(path)
		if err != nil {
			log.Println(err)
		}
	}

	return nil
}

// Creates a slice of each branch of the full path
// as individual paths to be iterated through.
func getDeletePaths(fullPath string) []string {

	paths := []string{}

	pathComponents := strings.Split(fullPath, "/")

	for i := range pathComponents {
		paths = append(paths, strings.Join(pathComponents[:len(pathComponents)-i], "/"))
	}

	return paths
}

func isValidHex(s string) bool {
	for _, b := range []byte(s) {
		if !(b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F') {
			return false
		}
	}
	return true
}
