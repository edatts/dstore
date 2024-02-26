package main

import (
	"log"

	"github.com/edatts/dstore/p2p"
)

func init() {
	// Maybe do some stuff here

}

func main() {
	// Spin up two nodes locally
	// node1 := NewTcpNode("localhost:3010")
	// node2 := NewTcpNode("localhost:3011")

	tr := p2p.NewTcpTransport(":3010")

	err := tr.ListenAndAccept()
	if err != nil {
		log.Fatal(err)
	}

	select {}
}
