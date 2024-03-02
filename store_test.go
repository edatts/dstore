package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestPathTransformFunc(t *testing.T) {
	r := bytes.NewReader([]byte("testKey"))
	CASPath, fileName, err := PathTransformFunc(r)
	if err != nil {
		t.Error(err)
	}
	filePath := CASPath + fileName
	expectedFilePath := "15291f67/d99ea7bc/578c3544/dadfbb99/1e66fa69cb36ff70fe30e798e111ff5f"

	if filePath != expectedFilePath {
		t.Errorf("have %s, want %s", filePath, expectedFilePath)
	}
}

func TestStore(t *testing.T) {

	opts := StoreOpts{
		PathTransformFunc:      PathTransformFunc,
		FileHashToFilePathFunc: FileHashToFilePathFunc,
	}

	store := NewStore(opts)

	fileContent := "File content."

	r := bytes.NewReader([]byte(fileContent))

	fileHash, err := store.writeStream(r)
	if err != nil {
		t.Error(err)
	}

	f, err := store.readStream(fileHash)
	if err != nil {
		t.Error(err)
	}

	fileBuf := make([]byte, 1024)

	n, err := f.Read(fileBuf)
	if err != nil {
		t.Error(err)
	}

	if string(fileBuf[:n]) != fileContent {
		t.Errorf("incorrect file content, expected %s, got %s", fileContent, string(fileBuf[:n]))
	}

	// Test reading file that is too big

	// Test reading with a buffered reader

	// Test deleting file with all parent directories empty

	// Test deleting file with a non-empty parent directory

}

func TestWriteStream(t *testing.T) {

	opts := StoreOpts{
		PathTransformFunc: PathTransformFunc,
	}

	store := NewStore(opts)

	r := bytes.NewReader([]byte("File content."))

	fileHash, err := store.writeStream(r)
	if err != nil {
		t.Error(err)
	}

	expectedFileHash := "cd655c0648230dd79b730daae566a148d982ebbc37c19bee4062a4e7c73e1fbc"

	if fileHash != expectedFileHash {
		t.Errorf("wrong fileHash, expected %s, got %s", expectedFileHash, fileHash)
	}

}

func TestReadStream(t *testing.T) {

	opts := StoreOpts{
		PathTransformFunc:      PathTransformFunc,
		FileHashToFilePathFunc: FileHashToFilePathFunc,
	}

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

func TestWrite(t *testing.T) {

	opts := StoreOpts{
		PathTransformFunc:      PathTransformFunc,
		FileHashToFilePathFunc: FileHashToFilePathFunc,
	}

	store := NewStore(opts)

	_ = store
}

func TestRead(t *testing.T) {

	opts := StoreOpts{
		PathTransformFunc:      PathTransformFunc,
		FileHashToFilePathFunc: FileHashToFilePathFunc,
	}

	store := NewStore(opts)

	// Test with invalid hash
	invalidHash := "he'sNotTheMassiahHe'sAVeryNaughtyHash!"
	_, err := store.Read(invalidHash)
	if err == nil {
		t.Error("missing error: invalid hash should produce error")
	}

	// Test with hash of file that does not exist
	nonExistentHash := sha256.Sum256([]byte("Not a real file"))
	nonExistentHashHex := hex.EncodeToString(nonExistentHash[:])
	_, err = store.Read(nonExistentHashHex)
	if err == nil {
		t.Error("missing error: hash for non-existent file should produce error")
	}

}

func TestGetDeletePaths(t *testing.T) {

	paths := getDeletePaths("first/second/another/final")
	expectedPaths := []string{
		"first/second/another/final",
		"first/second/another",
		"first/second",
		"first",
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
