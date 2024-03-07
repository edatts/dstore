package p2p

import (
	"encoding/gob"
	"fmt"
	"io"
	"log"
)

type Decoder interface {
	Decode(io.Reader, *Message) error
}

type GobDecoder struct {
}

func (d GobDecoder) Decode(r io.Reader, x *Message) error {
	return gob.NewDecoder(r).Decode(x)
}

type DefaultDecoder struct {
}

func (d DefaultDecoder) Decode(r io.Reader, msg *Message) error {
	buf := make([]byte, 2048)

	n, err := r.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to decode: %w", err)
	}

	log.Printf("Read %v bytes.\n", n)

	msg.Payload = buf[:n]

	return nil
}
