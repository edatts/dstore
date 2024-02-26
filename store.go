package main

import "io"

func DefaultTransformFunc(key string) string {
	return key
}

type StoreOpts struct {
	TransformFunc func(string) string
}

type Store struct {
	StoreOpts
}

func NewStore(opts StoreOpts) *Store {

	return &Store{}
}

func (s *Store) WriteStream(key string, r io.Reader) error {

	path := s.TransformFunc(key)

	return nil
}
