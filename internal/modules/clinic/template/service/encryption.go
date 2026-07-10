package service

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/iamarpitzala/acareca/internal/shared/crypto"
)

// Encryption errors
var (
	ErrInvalidEncryptionKey = errors.New("encryption key must be exactly 32 characters")
)

// IEncryptionService handles template encryption/decryption
type IEncryptionService interface {
	EncryptTemplate(html, css string) (htmlBlob, cssBlob []byte, err error)
	DecryptTemplate(htmlBlob, cssBlob []byte) (html, css string, err error)
	EncryptAndEncodeTemplate(html, css string) (htmlEncoded, cssEncoded string, err error)
	DecodeAndDecryptTemplate(htmlEncoded, cssEncoded string) (html, css string, err error)
}

type EncryptionService struct {
	key []byte
}

func NewEncryptionService(key string) IEncryptionService {
	return &EncryptionService{key: []byte(key)}
}

func (s *EncryptionService) EncryptTemplate(html, css string) ([]byte, []byte, error) {
	htmlBlob, err := crypto.EncryptAndCompress(html, s.key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encrypt HTML: %w", err)
	}

	cssBlob, err := crypto.EncryptAndCompress(css, s.key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encrypt CSS: %w", err)
	}

	return htmlBlob, cssBlob, nil
}

func (s *EncryptionService) DecryptTemplate(htmlBlob, cssBlob []byte) (string, string, error) {
	html, err := crypto.DecryptAndDecompress(htmlBlob, s.key)
	if err != nil {
		return "", "", fmt.Errorf("failed to decrypt HTML: %w", err)
	}

	css, err := crypto.DecryptAndDecompress(cssBlob, s.key)
	if err != nil {
		return "", "", fmt.Errorf("failed to decrypt CSS: %w", err)
	}

	return html, css, nil
}

func (s *EncryptionService) EncryptAndEncodeTemplate(html, css string) (string, string, error) {
	htmlBlob, cssBlob, err := s.EncryptTemplate(html, css)
	if err != nil {
		return "", "", err
	}

	return base64.StdEncoding.EncodeToString(htmlBlob),
		base64.StdEncoding.EncodeToString(cssBlob),
		nil
}

func (s *EncryptionService) DecodeAndDecryptTemplate(htmlEncoded, cssEncoded string) (string, string, error) {
	htmlBlob, err := base64.StdEncoding.DecodeString(htmlEncoded)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode HTML: %w", err)
	}

	cssBlob, err := base64.StdEncoding.DecodeString(cssEncoded)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode CSS: %w", err)
	}

	return s.DecryptTemplate(htmlBlob, cssBlob)
}
