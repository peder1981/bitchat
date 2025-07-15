package crypto

import (
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"
)

// DecryptWithPrivateKey descriptografa dados usando uma chave privada X25519
func DecryptWithPrivateKey(ciphertext []byte, nonce []byte, privateKey []byte) ([]byte, error) {
	// Verificar tamanhos
	if len(privateKey) != 32 {
		return nil, ErrInvalidPrivateKey
	}
	if len(nonce) != 24 {
		return nil, ErrInvalidNonce
	}

	// Converter para os tipos esperados pelo pacote nacl/box
	var nonceArray [24]byte
	copy(nonceArray[:], nonce)

	var privateKeyArray [32]byte
	copy(privateKeyArray[:], privateKey)

	// Gerar chave pública correspondente
	var publicKeyArray [32]byte
	curve25519.ScalarBaseMult(&publicKeyArray, &privateKeyArray)

	// Descriptografar
	decrypted, ok := box.Open(nil, ciphertext, &nonceArray, &publicKeyArray, &privateKeyArray)
	if !ok {
		return nil, ErrDecryptionFailed
	}

	return decrypted, nil
}
