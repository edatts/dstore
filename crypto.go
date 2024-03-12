package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"golang.org/x/crypto/scrypt"
)

func NewKey() ([]byte, error) {
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to derive new key: %s", err)
	}
	return buf, nil
}

// When deriving an AES key from a password we can provide a salt
// to recover a key that we have used before. Alternatively, we
// can provide a nil byte slice when we are deriving a key for
// the first time, the salt is randomly generated and returned
// with the derived key.
func DeriveKeyFromPass(password string, salt []byte) ([]byte, []byte, error) {
	var (
		N      int = 1048576
		r      int = 8
		p      int = 1
		keyLen int = 32
	)

	if salt == nil {
		salt = make([]byte, 32)
		if _, err := rand.Read(salt); err != nil {
			return nil, nil, fmt.Errorf("failed to generate salt: %w", err)
		}
	}

	key, err := scrypt.Key([]byte(password), salt, N, r, p, keyLen)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to derive key from password: %w", err)
	}

	return key, salt, nil
}

func encryptStream(dst io.Writer, src io.Reader, key []byte, salt []byte) (int, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, fmt.Errorf("failed to get new cipher: %w", err)
	}

	if _, err := dst.Write(salt); err != nil {
		return 0, fmt.Errorf("failed to write salt to dst: %w", err)
	}

	iv := make([]byte, block.BlockSize())
	if _, err = rand.Read(iv); err != nil {
		return 0, fmt.Errorf("failed to create iv: %w", err)
	}

	if n, err := dst.Write(iv); err != nil {
		return n, fmt.Errorf("failed writing to dst: %w", err)
	}

	stream := cipher.NewCTR(block, iv)

	return copyCTR(dst, src, stream)
}

func decryptStream(dst io.Writer, src io.Reader, key []byte) (int, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, fmt.Errorf("failed to get new cipher: %w", err)
	}

	// Read iv from file
	iv := make([]byte, block.BlockSize())
	if _, err := src.Read(iv); err != nil {
		return 0, fmt.Errorf("failed creating iv: %w", err)
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
			stream.XORKeyStream(buf, buf)
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

func Encrypt(dst io.Writer, src io.Reader, key []byte) (int, error) {
	// When encrypting without a password we will provide a random
	// salt that will be unused, it is only there to maintain
	// consistent structure of bytes in each file.
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return 0, fmt.Errorf("failed to generate salt: %w", err)
	}

	return encryptStream(dst, src, key, salt)
}

func Decrypt(dst io.Writer, src io.Reader, key []byte) (int, error) {
	// We don't need the salt for decryption here, so we will
	// just read it then discard it.

	// Read salt from file
	salt := make([]byte, 32)
	if _, err := src.Read(salt); err != nil {
		return 0, fmt.Errorf("failed to read salt from source: %w", err)
	}

	return decryptStream(dst, src, key)
}

func EncryptWithPassword(dst io.Writer, src io.Reader, password string) (int, error) {
	key, salt, err := DeriveKeyFromPass(password, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to derive key: %w", err)
	}

	return encryptStream(dst, src, key, salt)
}

func DecryptWithPassword(dst io.Writer, src io.Reader, password string) (int, error) {
	salt := make([]byte, 32)
	if _, err := src.Read(salt); err != nil {
		return 0, fmt.Errorf("failed to read salt from source: %w", err)
	}

	key, _, err := DeriveKeyFromPass(password, salt)
	if err != nil {
		return 0, fmt.Errorf("failed to derive key: %w", err)
	}

	return decryptStream(dst, src, key)
}

// Here we need to write the salt from the password derivation
// to the front of the file.
// func passwordEncryptStream(dst io.Writer, src io.Reader, password []byte) (int, error) {
// 	var (
// 		salt   []byte = make([]byte, 32)
// 		N      int    = 1048576
// 		r      int    = 8
// 		p      int    = 1
// 		keyLen int    = 32
// 	)

// 	if _, err := rand.Read(salt); err != nil {
// 		return 0, fmt.Errorf("failed generating salt: %w", err)
// 	}

// 	// Derive key from password
// 	key, err := scrypt.Key(password, salt, N, r, p, keyLen)
// 	if err != nil {
// 		return 0, fmt.Errorf("failed to derive key from password: %w", err)
// 	}

// 	// TODO: We need to find a clean way to write the salt to the file
// 	return encryptStream(dst, src, key, salt)
// }
