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

// Default decoder is a no-op decoder that reads the raw
// bytes from the source reader.
type DefaultDecoder struct {
}

// No-op decode function that simply reads the raw bytes
// into a byte slice.
func (d DefaultDecoder) Decode(r io.Reader, msg *Message) error {
	buf := make([]byte, 2048)

	n, err := r.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to decode: %w", err)
	}

	log.Printf("Read %v bytes.\n", n)

	msg.Content = buf[:n]

	return nil
}
