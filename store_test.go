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

	r := bytes.NewReader([]byte("File content."))

	err := store.WriteStream("testFolder", r)
	if err != nil {
		t.Error(err)
	}

}

func TestWriteStream(t *testing.T) {

	opts := StoreOpts{
		PathTransformFunc: PathTransformFunc,
	}

	store := NewStore(opts)

	r := bytes.NewReader([]byte("File content."))

	err := store.WriteStream(r)
	if err != nil {
		t.Error(err)
	}

}
