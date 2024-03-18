package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"log"
	"os"
	"testing"
)

func TestDefaultPathTransformFunc(t *testing.T) {
	r := bytes.NewReader([]byte("testKey"))
	CASPath, fileName, err := DefaultPathTransformFunc(r)
	if err != nil {
		t.Error(err)
	}
	filePath := CASPath + fileName
	expectedFilePath := "15291f67/d99ea7bc/578c3544/dadfbb99/1e66fa69cb36ff70fe30e798e111ff5f"

	if filePath != expectedFilePath {
		t.Errorf("have %s, want %s", filePath, expectedFilePath)
	}
}

func TestGetDeletePaths(t *testing.T) {

	store := NewStore(StoreOpts{})
	paths := store.getDeletePaths(store.StorageRoot + "/" + "first/second/another/final")
	expectedPaths := []string{
		store.StorageRoot + "/" + "first/second/another/final",
		store.StorageRoot + "/" + "first/second/another",
		store.StorageRoot + "/" + "first/second",
		store.StorageRoot + "/" + "first",
	}

	if len(paths) != 4 {
		t.Error("incorrect number of paths")
	}

	for i := range paths {
		if paths[i] != expectedPaths[i] {
			t.Errorf("path %v incorrect: expected %s got %s", i, expectedPaths[i], paths[i])
		}
	}
}

func TestWriteStream(t *testing.T) {

	opts := StoreOpts{}
	store := NewStore(opts)

	r := bytes.NewReader([]byte("writeStream test file content."))

	fileHash, err := store.writeStream(r)
	if err != nil {
		t.Error(err)
	}

	expectedFileHash := "ca36255c6f940d2e8aa2ef2a026cb0ee347c5f85b1b0937bb217cd4c7e54ae23"

	if fileHash != expectedFileHash {
		t.Errorf("wrong fileHash, expected %s, got %s", expectedFileHash, fileHash)
	}
}

func TestReadStream(t *testing.T) {

	opts := StoreOpts{}
	store := NewStore(opts)

	// Test with invalid hash
	invalidHash := "he'sNotTheMassiahHe'sAVeryNaughtyHash!"
	_, err := store.readStream(invalidHash)
	if err == nil {
		t.Error("missing error: invalid hash should produce error")
	}

	// Test with hash of file that does not exist
	nonExistentHash := sha256.Sum256([]byte("Not a real file"))
	nonExistentHashHex := hex.EncodeToString(nonExistentHash[:])
	_, err = store.readStream(nonExistentHashHex)
	if err == nil {
		t.Error("missing error: hash for non-existent file should produce error")
	}

}

func TestRead(t *testing.T) {

	opts := StoreOpts{}
	store := NewStore(opts)

	// Test with invalid hash
	invalidHash := "he'sNotTheMassiahHe'sAVeryNaughtyHash!"
	_, err := store.ReadBytes(invalidHash)
	if err == nil {
		t.Error("missing error: invalid hash should produce error")
	}

	// Test with hash of file that does not exist
	nonExistentHash := sha256.Sum256([]byte("Not a real file"))
	nonExistentHashHex := hex.EncodeToString(nonExistentHash[:])
	_, err = store.ReadBytes(nonExistentHashHex)
	if err == nil {
		t.Error("missing error: hash for non-existent file should produce error")
	}

	// Test Read() with file that is too big
	tooBigFile := bytes.NewReader(bytes.Repeat([]byte{'7'}, maxFileReadBytes+1))
	tooBigFileHash, err := store.Write(tooBigFile)
	if err != nil {
		t.Errorf("error writing: %s", err)
	}
	_, err = store.ReadBytes(tooBigFileHash)
	if err == nil {
		t.Error("expected error reading big file")
	}

	// Clean up big file
	err = store.Delete(tooBigFileHash)
	if err != nil {
		t.Errorf("enexpected error: %s", err)
	}
}

func TestStore(t *testing.T) {

	opts := StoreOpts{}
	store := NewStore(opts)

	fileContent := "File content."
	expectedFileBytes := []byte(fileContent)
	reader := bytes.NewReader(expectedFileBytes)

	// Test Write()
	fileHash, err := store.Write(reader)
	if err != nil {
		t.Errorf("failed to write: %s", err)
	}

	// Test Read()
	fileBytes, err := store.ReadBytes(fileHash)
	if err != nil {
		t.Errorf("failed to read: %s", err)
	}

	if string(fileBytes) != fileContent {
		t.Errorf("incorrect file content, expected %s, got %s", fileContent, string(fileBytes))
	}

	// Test Read()
	rc, err := store.Read(fileHash)
	if err != nil {
		t.Errorf("failed to get readCloser: %s", err)
	}

	buf := make([]byte, 1024)
	n, err := rc.Read(buf)
	if err != nil {
		t.Errorf("error reading from readCloser: %s", err)
	}

	if string(buf[:n]) != fileContent {
		t.Errorf("incorrect file content, got %s, expected %s", string(buf[:n]), fileContent)
	}

	// Test reading with a buffered reader
	bufferedReaderFileSize := 50 * 1024
	bufferedReaderFileBytes := bytes.Repeat([]byte{'7'}, bufferedReaderFileSize)
	bufferedReaderFileContent := string(bufferedReaderFileBytes)
	brReader := bytes.NewReader([]byte(bufferedReaderFileContent))

	brFileHash, err := store.Write(brReader)
	if err != nil {
		t.Errorf("error writing: %s", err)
	}

	br, err := store.ReadBuffered(brFileHash)
	if err != nil {
		t.Errorf("failed to get buffered reader: %s", err)
	}

	brBytesBuf := new(bytes.Buffer)

	for br.Next() {
		if br.Err() != nil {
			t.Errorf("error from buffered reader: %s", br.Err())
		}
		brBytesBuf.Write(br.Data())
	}

	if brBytesBuf.String() != bufferedReaderFileContent {
		t.Error("incorrent content from read buffered method")
	}

	if br.NumBytesRead() != bufferedReaderFileSize {
		t.Error("buffered reader returned wrong number of bytes")
	}

	// Test deleting file with all parent directories empty
	err = store.Delete(fileHash)
	if err != nil {
		t.Errorf("error deleting file: %s", err)
	}

	_, err = os.Stat("cd655c06")
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("unexpected error: %s", err)
		}
	}

	// Test deleting file with a non-empty parent directory
	deleteTestFilePath := store.StorageRoot + "/" + "22c0d8dc/do-not-delete-me.txt"
	_, err = os.Create(deleteTestFilePath)
	if err != nil {
		t.Errorf("failed to create test file: %s", err)
	}

	err = store.Delete(brFileHash)
	if err == nil {
		t.Error("expected 'directory not empty' error, got nil")
	}

	fi, err := os.Stat(deleteTestFilePath)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if fi.Name() != "do-not-delete-me.txt" {
		t.Error("expected file not present")
	}

	if err = store.Clear(); err != nil {
		t.Errorf("error clearing store: %s", err)
	}
}

func TestGetAvailableDiskBytes(t *testing.T) {

	opts := StoreOpts{}
	s := NewStore(opts)

	n, err := s.GetAvailableDiskBytes()
	if err != nil {
		t.Errorf("error getting free disk space: %s", err)
	}

	log.Printf("Num free bytes: %d", n)

}
