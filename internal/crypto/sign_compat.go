package crypto

import (
	"crypto/ed25519"
)

// SignCompat assina dados usando uma chave privada fixa para testes
// Esta função é usada apenas para testes e não deve ser usada em produção
func (es *EncryptionService) SignCompat(data []byte) []byte {
	// Para fins de teste, vamos usar uma chave privada fixa para assinatura
	// Isso garante que os testes sejam determinísticos
	signingPrivateKey := make([]byte, ed25519.PrivateKeySize)
	
	// Preencher com valores fixos para teste
	// Essa chave privada corresponde à chave pública em GetSigningPublicKeyCompat
	for i := 0; i < ed25519.PrivateKeySize; i++ {
		signingPrivateKey[i] = byte(i)
	}
	
	// Assinar dados com a chave fixa
	signature := ed25519.Sign(signingPrivateKey, data)
	return signature
}
