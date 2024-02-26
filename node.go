package main

import ()

// A node will communicate with other nodes to share stored data.
// It will also expose an API for interacting with the storage system.
type node struct {
}

func NewTcpNode(addr string) *node {
	return &node{}
}
