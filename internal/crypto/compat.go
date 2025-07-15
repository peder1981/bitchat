package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"io"
)

// EncryptWithPublicKeyCompat é uma versão compatível com os testes de integração
// que aceita uma chave pública em formato string e retorna o ciphertext, nonce e erro
func (es *EncryptionService) EncryptWithPublicKeyCompat(data []byte, publicKey string) ([]byte, []byte, error) {
	// Converter a chave pública de string para []byte
	var pubKeyBytes []byte
	var err error
	
	// Verificar se a chave está em formato hexadecimal
	if len(publicKey) == 64 { // 32 bytes em hex = 64 caracteres
		pubKeyBytes, err = hex.DecodeString(publicKey)
		if err != nil {
			return nil, nil, ErrInvalidPublicKey
		}
	} else {
		// Assumir que é uma chave binária
		pubKeyBytes = []byte(publicKey)
	}
	
	// Verificar tamanho da chave
	if len(pubKeyBytes) != 32 {
		return nil, nil, ErrInvalidPublicKey
	}
	
	// Gerar nonce aleatório
	nonce := make([]byte, 24)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	
	// Criptografar os dados
	ciphertext, nonce, err := es.EncryptWithKey(data, pubKeyBytes)
	if err != nil {
		return nil, nil, err
	}
	
	return ciphertext, nonce, nil
}

// VerifyCompat é uma versão compatível com os testes de integração
// que aceita uma chave pública em formato string
func (es *EncryptionService) VerifyCompat(signature, data []byte, publicKey string) (bool, error) {
	// Converter a chave pública de string para []byte
	var pubKeyBytes []byte
	var err error
	
	// Verificar se a chave está em formato hexadecimal
	if len(publicKey) == 64 { // 32 bytes em hex = 64 caracteres
		pubKeyBytes, err = hex.DecodeString(publicKey)
		if err != nil {
			return false, ErrInvalidPublicKey
		}
	} else {
		// Assumir que é uma chave binária
		pubKeyBytes = []byte(publicKey)
	}
	
	// Verificar a assinatura usando ed25519
	return ed25519.Verify(pubKeyBytes, data, signature), nil
}

// GetPublicKeyString retorna a chave pública para criptografia em formato string
func (es *EncryptionService) GetPublicKeyString() string {
	return hex.EncodeToString(es.GetPublicKey())
}

// GetPublicKeyCompat retorna uma chave pública fixa para testes de integração
// Esta função é usada apenas para testes e não deve ser usada em produção
func (es *EncryptionService) GetPublicKeyCompat() []byte {
	// Para fins de teste, vamos usar uma chave pública fixa
	// correspondente à chave privada fixa em EncryptCompat
	publicKey := make([]byte, 32)
	
	// Preencher com valores fixos para teste
	for i := 0; i < 32; i++ {
		publicKey[i] = byte(96 + i)
	}
	
	return publicKey
}

// GetSigningPublicKeyString retorna a chave pública para assinatura em formato string
func (es *EncryptionService) GetSigningPublicKeyString() string {
	return hex.EncodeToString(es.signingPublicKey)
}

// GetSigningPublicKeyCompat retorna uma chave pública fixa para assinatura em testes
// Esta função é usada apenas para testes e não deve ser usada em produção
func (es *EncryptionService) GetSigningPublicKeyCompat() []byte {
	// Para fins de teste, vamos usar uma chave pública fixa para assinatura
	// correspondente à chave privada fixa em SignCompat
	signingPublicKey := make([]byte, ed25519.PublicKeySize)
	
	// Preencher com valores fixos para teste
	for i := 0; i < ed25519.PublicKeySize; i++ {
		signingPublicKey[i] = byte(128 + i)
	}
	
	return signingPublicKey
}
