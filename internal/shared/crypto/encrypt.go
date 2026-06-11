package crypto

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

func EncryptAndCompress(plainText string, secretKey []byte) ([]byte, error) {
	if len(secretKey) != 32 {
		return nil, fmt.Errorf("crypto: secret key must be exactly 32 bytes")
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte(plainText)); err != nil {
		return nil, err
	}
	_ = gz.Close()
	compressed := buf.Bytes()

	block, err := aes.NewCipher(secretKey)
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return aesGCM.Seal(nonce, nonce, compressed, nil), nil
}

func DecryptAndDecompress(cipherText []byte, secretKey []byte) (string, error) {
	if len(secretKey) != 32 {
		return "", fmt.Errorf("crypto: secret key must be exactly 32 bytes")
	}

	block, err := aes.NewCipher(secretKey)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(cipherText) < nonceSize {
		return "", fmt.Errorf("crypto: ciphertext too short")
	}

	nonce, cipherText := cipherText[:nonceSize], cipherText[nonceSize:]
	compressed, err := aesGCM.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: decryption failed: %w", err)
	}

	gr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return "", fmt.Errorf("crypto: gzip reader failed: %w", err)
	}
	defer gr.Close()

	plain, err := io.ReadAll(gr)
	if err != nil {
		return "", fmt.Errorf("crypto: decompression failed: %w", err)
	}

	return string(plain), nil
}
