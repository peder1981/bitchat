package crypto

import (
	"crypto/rand"
	"io"

	"golang.org/x/crypto/curve25519"
)

// curve25519PublicKey gera uma chave pública a partir de uma chave privada usando Curve25519
func curve25519PublicKey(publicKey, privateKey *[32]byte) {
	curve25519.ScalarBaseMult(publicKey, privateKey)
}

// GenerateX25519KeyPair gera um par de chaves X25519
func GenerateX25519KeyPair() ([]byte, []byte, error) {
	// Gerar chave privada aleatória
	privateKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, privateKey); err != nil {
		return nil, nil, err
	}

	// Ajustar bits conforme recomendado para X25519
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	// Converter para array
	var privateKeyArray [32]byte
	copy(privateKeyArray[:], privateKey)

	// Gerar chave pública
	var publicKeyArray [32]byte
	curve25519.ScalarBaseMult(&publicKeyArray, &privateKeyArray)

	// Converter de volta para slice
	publicKey := make([]byte, 32)
	copy(publicKey, publicKeyArray[:])

	return privateKey, publicKey, nil
}
