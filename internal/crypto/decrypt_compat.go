package crypto

import (
	"errors"
	"golang.org/x/crypto/nacl/box"
)

// Erros de criptografia
var (
	ErrInvalidPrivateKey = errors.New("chave privada inválida")
	ErrInvalidNonce      = errors.New("nonce inválido")
)

// DecryptWithPrivateKeyCompat é uma função de compatibilidade para os testes de integração
// que aceita e retorna os tipos esperados pelos testes
// Importante: Para o NaCl box, precisamos da chave pública do remetente e da chave privada do destinatário
func DecryptWithPrivateKeyCompat(ciphertext []byte, nonce []byte, privateKey []byte, senderPublicKey []byte) ([]byte, error) {
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

	// Verificar se a chave pública do remetente é válida
	if len(senderPublicKey) != 32 {
		return nil, ErrInvalidPublicKey
	}

	// Preparar a chave pública do remetente para o formato esperado por box.Open
	var publicKeyArray [32]byte
	copy(publicKeyArray[:], senderPublicKey)

	// Descriptografar usando a chave pública do remetente e a chave privada do destinatário
	// Na criptografia NaCl box:
	// - Para criptografar: box.Seal(nil, plaintext, nonce, recipientPubKey, senderPrivKey)
	// - Para descriptografar: box.Open(nil, ciphertext, nonce, senderPubKey, recipientPrivKey)
	// Portanto, precisamos trocar a ordem das chaves públicas/privadas para descriptografar corretamente
	decrypted, ok := box.Open(nil, ciphertext, &nonceArray, &publicKeyArray, &privateKeyArray)
	if !ok {
		return nil, ErrDecryptionFailed // Definido em encryption.go
	}

	return decrypted, nil
}
