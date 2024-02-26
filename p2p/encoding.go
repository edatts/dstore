package p2p

import (
	"encoding/gob"
	"errors"
	"io"
	"log"
)

var ErrFailedToDecode = errors.New("failed to decode")

type Decoder interface {
	Decode(io.Reader, any) error
}

type GobDecoder struct {
}

func (d GobDecoder) Decode(r io.Reader, x any) error {
	return gob.NewDecoder(r).Decode(x)
}

type DefaultDecoder struct {
}

func (d DefaultDecoder) Decode(r io.Reader, x any) error {
	buf := make([]byte, 2048)

	n, err := r.Read(buf)
	if err != nil {
		return ErrFailedToDecode
	}

	log.Printf("Read %v bytes.\n", n)

	return nil
}
