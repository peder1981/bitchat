package service

import (
	"github.com/permissionlesstech/bitchat/pkg/utils"
	"github.com/pierrec/lz4/v4"
)

// CompressionService fornece funcionalidades de compressão e descompressão
type CompressionService struct {
	compressionLevel lz4.CompressionLevel
}

// NewCompressionService cria um novo serviço de compressão
func NewCompressionService(level int) *CompressionService {
	// Converter nível inteiro para CompressionLevel do lz4
	var compressionLevel lz4.CompressionLevel
	
	switch level {
	case 0:
		compressionLevel = lz4.Fast
	case 1:
		compressionLevel = lz4.Level1
	case 2:
		compressionLevel = lz4.Level2
	case 3:
		compressionLevel = lz4.Level3
	case 4:
		compressionLevel = lz4.Level4
	case 5:
		compressionLevel = lz4.Level5
	case 6:
		compressionLevel = lz4.Level6
	case 7:
		compressionLevel = lz4.Level7
	case 8:
		compressionLevel = lz4.Level8
	case 9:
		compressionLevel = lz4.Level9
	default:
		compressionLevel = lz4.Level1 // Nível padrão
	}
	
	return &CompressionService{
		compressionLevel: compressionLevel,
	}
}

// Compress comprime dados usando o algoritmo LZ4
func (cs *CompressionService) Compress(data []byte, mimeType string) ([]byte, error) {
	// Verificar se o tipo de conteúdo deve ser comprimido
	if !utils.ShouldCompress(mimeType) {
		return data, nil
	}
	
	// Usar a função de compressão do pacote utils
	return utils.CompressData(data)
}

// Decompress descomprime dados comprimidos com LZ4
func (cs *CompressionService) Decompress(compressedData []byte, mimeType string) ([]byte, error) {
	// Usar a função de descompressão do pacote utils
	return utils.DecompressData(compressedData)
}
