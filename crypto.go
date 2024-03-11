package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

func NewKey() ([]byte, error) {
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to derive new key: %s", err)
	}
	return buf, nil
}

func encryptStream(dst io.Writer, src io.Reader, key []byte) (int, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, fmt.Errorf("failed to get new cipher: %w", err)
	}

	iv := make([]byte, block.BlockSize())
	if _, err = rand.Read(iv); err != nil {
		return 0, fmt.Errorf("failed to create iv: %w", err)
	}

	stream := cipher.NewCTR(block, iv)

	return copyCTR(dst, src, stream)
}

func copyCTR(dst io.Writer, src io.Reader, stream cipher.Stream) (int, error) {
	var (
		buf    = make([]byte, 32*1024)
		nBytes int
	)

	for {
		n, err := src.Read(buf)
		if n > 0 {
			stream.XORKeyStream(buf[:n], buf)
			if _, err := dst.Write(buf[:n]); err != nil {
				return nBytes, fmt.Errorf("failed writing to dst: %w", err)
			}
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return nBytes, fmt.Errorf("failed reading from src: %w", err)
		}

		nBytes += n
	}

	return nBytes, nil
}

func decryptStream(dst io.Writer, src io.Reader, key []byte) (int, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, fmt.Errorf("failed to get new cipher: %w", err)
	}

	iv := make([]byte, block.BlockSize())
	if _, err := rand.Read(iv); err != nil {
		return 0, fmt.Errorf("failed creating iv: %w", err)
	}

	stream := cipher.NewCTR(block, iv)

	return copyCTR(dst, src, stream)
}

func passwordEncryptoStream() {

}
