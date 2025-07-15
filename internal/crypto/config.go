package crypto

// EncryptionConfig contém configurações para o serviço de criptografia
type EncryptionConfig struct {
	KeysDir string // Diretório para armazenar chaves persistentes
	UseEphemeralOnly bool // Se verdadeiro, não persiste chaves no disco
	KeyStorePath string // Caminho para armazenamento de chaves (compatível com testes)
}
