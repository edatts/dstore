package main

import (
	"bytes"
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

	fileHash, err := store.WriteStream(r)
	if err != nil {
		t.Error(err)
	}

	f, err := store.ReadStream(fileHash)
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
}

func TestWriteStream(t *testing.T) {

	opts := StoreOpts{
		PathTransformFunc: PathTransformFunc,
	}

	store := NewStore(opts)

	r := bytes.NewReader([]byte("File content."))

	fileHash, err := store.WriteStream(r)
	if err != nil {
		t.Error(err)
	}

	expectedFileHash := "cd655c0648230dd79b730daae566a148d982ebbc37c19bee4062a4e7c73e1fbc"

	if fileHash != expectedFileHash {
		t.Errorf("wrong fileHash, expected %s, got %s", expectedFileHash, fileHash)
	}

}
