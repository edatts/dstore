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
	blockSize             int    = 8
	maxFileReadBytes      int    = 200 * 1024 * 1024 // 200 MiB
	bufferedReaderBufSize int    = 16 * 1024         // 16 KiB
	defaultStoragePath    string = "storage"
)

type Metadata struct {
	FileSize int64
}

// Creates the file path from the data streamed through the
// input reader, this way the path and file name are entirely
// derived from the content of the file to be stored.
func DefaultPathTransformFunc(r io.Reader) (string, string, error) {
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
func DefaultFileHashToFilePathFunc(fileHash string) (string, error) {

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
	// Storage root is the path to the directory where all the
	// child directories and files will be stored.
	StorageRoot string
}

type Store struct {
	StoreOpts
}

func NewStore(opts StoreOpts) *Store {
	if len(opts.StorageRoot) == 0 {
		opts.StorageRoot = defaultStoragePath
	}
	if opts.PathTransformFunc == nil {
		opts.PathTransformFunc = DefaultPathTransformFunc
	}
	if opts.FileHashToFilePathFunc == nil {
		opts.FileHashToFilePathFunc = DefaultFileHashToFilePathFunc
	}
	return &Store{
		opts,
	}
}

func (s *Store) Clear() error {
	return os.RemoveAll(s.StorageRoot)
}

func (s *Store) ReadStream(fileHash string) (io.ReadCloser, error) {

	if exists, err := s.fileExists(fileHash); !exists {
		if err != nil {
			return nil, fmt.Errorf("could not check if file exists: %w", err)
		}
		return nil, fmt.Errorf("file not found")
	}

	rc, err := s.readStream(fileHash)
	if err != nil {
		return nil, err
	}

	return rc, nil
}

// Read will read the file into a byte slice and return it.
// If the requested file is too big then Read() will return
// an error. An error will also be returned if the requested
// file cannot be found.
func (s *Store) Read(fileHash string) ([]byte, error) {

	// Get file path
	filePath, err := s.FileHashToFilePathFunc(fileHash)
	if err != nil {
		return nil, fmt.Errorf("failed to convert hash to file path: %w", err)
	}

	fullPath := s.StorageRoot + "/" + filePath

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

	filePath, err := s.FileHashToFilePathFunc(fileHash)
	if err != nil {
		return nil, err
	}

	fullPath := s.StorageRoot + "/" + filePath

	f, err := os.Open(fullPath)
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

	// Get file hash before creating full path.
	fileHash := strings.Join(strings.Split(path, "/"), "") + CASFileName

	fullPath := s.StorageRoot + "/" + path

	err = os.MkdirAll(fullPath, os.ModePerm)
	if err != nil {
		return "", err
	}

	// _, err = exec.Command("ls", "-al").Output()
	// if err != nil {
	// 	return "", err
	// }

	_, err = exec.Command("mv", tempFilePath, fullPath+CASFileName).Output()
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

	// Check for file
	if exists, err := s.fileExists(fileHash); !exists {
		if err != nil {
			return fmt.Errorf("could not check if file exists: %w", err)
		}
		return fmt.Errorf("file not found")
	}

	// Get file path
	filePath, err := s.FileHashToFilePathFunc(fileHash)
	if err != nil {
		return fmt.Errorf("failed to convert hash to file path: %w", err)
	}

	fullPath := s.StorageRoot + "/" + filePath

	// Delete file and any empty directories
	paths := s.getDeletePaths(fullPath)

	for _, path := range paths {
		err = os.Remove(path)
		if err != nil {
			return err
		}
	}

	return nil
}

// Creates a slice of each branch of the full path
// as individual paths to be iterated through.
func (s *Store) getDeletePaths(fullPath string) []string {
	paths := []string{}

	// Strip off the storage root first
	fullPath = strings.Replace(fullPath, s.StorageRoot+"/", "", 1)

	pathComponents := strings.Split(fullPath, "/")

	for i := range pathComponents {
		paths = append(paths, s.StorageRoot+"/"+strings.Join(pathComponents[:len(pathComponents)-i], "/"))
	}

	return paths
}

// Returns true if the corresponding file exists, false if the
// file does not exist, and false and an error if there was
// some other error.
func (s *Store) fileExists(fileHash string) (bool, error) {

	// Get file path
	filePath, err := s.FileHashToFilePathFunc(fileHash)
	if err != nil {
		return false, fmt.Errorf("failed to convert hash to file path: %w", err)
	}

	fullPath := s.StorageRoot + "/" + filePath

	// Check if file exists.
	_, err = os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if file exists: %w", err)
	}

	return true, nil
}

func isValidHex(s string) bool {
	for _, b := range []byte(s) {
		if !(b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F') {
			return false
		}
	}
	return true
}

func (s *Store) GetFileSize(fileHash string) (int64, error) {
	filePath, err := s.FileHashToFilePathFunc(fileHash)
	if err != nil {
		return 0, fmt.Errorf("failed to get path from file has: %w", err)
	}

	fi, err := os.Stat(s.StorageRoot + "/" + filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}

	return fi.Size(), nil
}

type BufferedReader struct {
	buf []byte
	r   io.ReadCloser
	e   error
	n   int
}

// Next() returns true if there is more data and false
// otherwise. The first call of Next() prepares the
// first chunk of data.
func (b *BufferedReader) Next() bool {
	n, err := b.r.Read(b.buf)
	if err != nil {
		if err == io.EOF {
			b.r.Close()
			return false
		}
		b.e = err
	}
	if n == 0 {
		b.r.Close()
		return false
	}

	if n < len(b.buf) {
		b.buf = b.buf[:n]
	}

	b.n += n

	return true
}

// Data() returns the current chunk of buffered data.
// Next() should be called before calling Data(), calling
// Data() before Next() will return an empty byte slice.
func (b *BufferedReader) Data() []byte {
	return b.buf
}

// Returns encountered errors. Does not return EOF as this
// is handled internally by the BufferedReader.
func (b *BufferedReader) Err() error {
	return b.e
}

// Returns the number of bytes read so far
func (b *BufferedReader) NumBytesRead() int {
	return b.n
}

// ReadBuffered() returns a pointer to a BufferedReader,
// which can be used to fetch the stored file in chunks.
// An error is returned if the requested file cannot be
// found.
func (s *Store) ReadBuffered(fileHash string) (*BufferedReader, error) {

	// Check for file
	if exists, err := s.fileExists(fileHash); !exists {
		if err != nil {
			return nil, fmt.Errorf("could not check if file exists: %w", err)
		}
		return nil, fmt.Errorf("file not found")
	}

	r, err := s.readStream(fileHash)
	if err != nil {
		return nil, err
	}

	br := &BufferedReader{
		buf: make([]byte, bufferedReaderBufSize),
		r:   r,
	}

	return br, nil
}
