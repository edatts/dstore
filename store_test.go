package main

import (
	"bytes"
	"os"
	"testing"
)

func TestCASPathTransformFunc(t *testing.T) {
	key := "testKey"
	CASPath := CASPathTransformFunc(key)
	expectedCASPath := "1529/1f67/d99e/a7bc/"

	if CASPath != expectedCASPath {
		t.Errorf("have %s, want %s", CASPath, expectedCASPath)
	}
}

func TestStore(t *testing.T) {

	opts := StoreOpts{
		PathTransformFunc: DefaultPathTransformFunc,
	}

	store := NewStore(opts)

	r := bytes.NewReader([]byte("File content."))

	err := store.WriteStream("testFolder", r)
	if err != nil {
		t.Error(err)
	}

	if _, err := os.Open("testFolder/testFileName"); err != nil {
		t.Error(err)
	}
}
