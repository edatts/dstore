package main

import (
	"bytes"
	"testing"
)

func TestCASPathTransformFunc(t *testing.T) {
	key := "testKey"
	CASPath := CASPathTransformFunc(key)
	expectedCASPath := "24af/da34/e3f7/4e54/b61a/8e4c/be92/1650/"

	if CASPath != expectedCASPath {
		t.Errorf("have %s, want %s", CASPath, expectedCASPath)
	}
}

// func TestStore(t *testing.T) {

// 	opts := StoreOpts{
// 		PathTransformFunc: CASPathTransformFunc,
// 	}

// 	store := NewStore(opts)

// 	r := bytes.NewReader([]byte("File content."))

// 	err := store.WriteStream("testFolder", r)
// 	if err != nil {
// 		t.Error(err)
// 	}

// }

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
