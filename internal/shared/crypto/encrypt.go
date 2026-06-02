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
