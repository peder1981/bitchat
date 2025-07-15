package crypto

// GenerateKeyPairCompat é uma função de compatibilidade para os testes de integração
// que gera um par de chaves X25519
func GenerateKeyPairCompat() ([]byte, []byte, error) {
	// Usar a implementação existente
	return GenerateKeyPair()
}
