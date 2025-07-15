package utils

import (
	"bytes"
	"errors"
	"strings"

	"github.com/pierrec/lz4/v4"
)

// CompressionService implementa compressão e descompressão LZ4
type CompressionService struct {
	// Configurações de compressão
	compressionLevel lz4.CompressionLevel
}

// NewCompressionService cria um novo serviço de compressão
func NewCompressionService() *CompressionService {
	return &CompressionService{
		compressionLevel: lz4.Fast, // Nível padrão de compressão
	}
}

// Compress comprime dados usando LZ4
func (cs *CompressionService) Compress(data []byte, contentType string) ([]byte, error) {
	// Verificar se o conteúdo deve ser comprimido
	if !cs.shouldCompress(data, contentType) {
		return data, nil
	}

	// Criar buffer para dados comprimidos
	var compressedBuf bytes.Buffer
	
	// Criar compressor LZ4
	zw := lz4.NewWriter(&compressedBuf)
	
	// Configurar nível de compressão
	zw.Apply(lz4.CompressionLevelOption(cs.compressionLevel))
	
	// Comprimir dados
	if _, err := zw.Write(data); err != nil {
		return nil, err
	}
	
	// Finalizar compressão
	if err := zw.Close(); err != nil {
		return nil, err
	}
	
	// Retornar dados comprimidos
	return compressedBuf.Bytes(), nil
}

// Decompress descomprime dados usando LZ4
func (cs *CompressionService) Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("dados vazios")
	}
	
	// Criar buffer para dados descomprimidos
	var decompressedBuf bytes.Buffer
	
	// Criar descompressor LZ4
	zr := lz4.NewReader(bytes.NewReader(data))
	
	// Descomprimir dados
	if _, err := decompressedBuf.ReadFrom(zr); err != nil {
		return nil, err
	}
	
	// Retornar dados descomprimidos
	return decompressedBuf.Bytes(), nil
}

// SetCompressionLevel define o nível de compressão
func (cs *CompressionService) SetCompressionLevel(level lz4.CompressionLevel) {
	cs.compressionLevel = level
}

// shouldCompress determina se um tipo de conteúdo deve ser comprimido
func (cs *CompressionService) shouldCompress(data []byte, contentType string) bool {
	// Não comprimir dados pequenos
	if len(data) < 100 {
		return false
	}
	
	// Não comprimir tipos de conteúdo já comprimidos
	compressedTypes := []string{
		"image/", "audio/", "video/",
		"application/zip", "application/gzip", "application/x-rar",
		"application/x-7z", "application/x-xz", "application/x-bzip",
	}
	
	for _, t := range compressedTypes {
		if strings.HasPrefix(contentType, t) {
			return false
		}
	}
	
	return true
}
