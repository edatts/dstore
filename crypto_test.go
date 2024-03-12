package main

import (
	"bytes"
	"log"
	"testing"
)

// func TestDeriveKeyFromPassword(t *testing.T) {

// }

func TestPasswordEncryptAndDecrypt(t *testing.T) {

	password := "TheMostSecurePasswordEver.1337"

	secretContent := "My super secret file content"
	secretFileBytes := []byte(secretContent)

	src := bytes.NewReader(secretFileBytes)
	encryptedFile := new(bytes.Buffer)

	n, err := EncryptWithPassword(encryptedFile, src, password)
	if err != nil {
		t.Errorf("error encrypting file: %s", err)
	}

	log.Printf("Encrypted (%d) bytes.", n)

	encryptedFileCopy := make([]byte, encryptedFile.Len())
	copy(encryptedFileCopy, encryptedFile.Bytes())

	decryptedFile := new(bytes.Buffer)

	// Test with wrong password
	n, err = DecryptWithPassword(decryptedFile, encryptedFile, "The wrong password")
	if err != nil {
		t.Errorf("failed to decrypt: %s", err)
	}

	log.Printf("Decrypted (%d) bytes.", n)

	if decryptedFile.String() != string(secretFileBytes) {
		log.Printf("Woohoo, file is different")
	}

	decryptedFile2 := new(bytes.Buffer)

	// Test with correct password
	n, err = DecryptWithPassword(decryptedFile2, bytes.NewReader(encryptedFileCopy), password)
	if err != nil {
		t.Errorf("failed to decrypt: %s", err)
	}

	log.Printf("Decrypted (%d) bytes.", n)

	if decryptedFile2.String() != secretContent {
		t.Errorf("File is different after decryption, expected %v, got %v", secretContent, decryptedFile2.String())
	}
}

// func TestEncryptAndDecrypt(t *testing.T) {

// }
