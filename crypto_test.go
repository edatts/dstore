package main

import (
	"bytes"
	"log"
	"testing"
)

func TestEncryptAndDecrypt(t *testing.T) {

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

	decryptedFile := new(bytes.Buffer)

	// Test with wrong password
	// DecryptWithPassword(decryptedFile, dst, "The wrong password")

	// if string(decryptedFile.Bytes()) != string(secretFileBytes) {
	// 	log.Printf("Woohoo, file is different")
	// }

	// Test with correct password
	DecryptWithPassword(decryptedFile, encryptedFile, password)

	log.Printf("Decrypted (%d) bytes.", n)

	if decryptedFile.String() != secretContent {
		t.Errorf("File is different after decryption, expected %v, got %v", secretContent, string(decryptedFile.Bytes()))
	}
}
