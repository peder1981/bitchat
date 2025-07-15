package crypto

import (
	"crypto/rand"
	"io"

	"golang.org/x/crypto/curve25519"
)

// GenerateKeyPair gera um novo par de chaves X25519 para criptografia
func GenerateKeyPair() (publicKey []byte, privateKey []byte, err error) {
	// Gerar chave privada aleatória
	privateKey = make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, privateKey); err != nil {
		return nil, nil, err
	}

	// Ajustar bits conforme especificação X25519
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	// Derivar chave pública
	var privKey [32]byte
	var pubKey [32]byte
	copy(privKey[:], privateKey)
	curve25519.ScalarBaseMult(&pubKey, &privKey)

	return pubKey[:], privateKey, nil
}
