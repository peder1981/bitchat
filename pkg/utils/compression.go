package utils

import (
	"bytes"
	"io"

	"github.com/pierrec/lz4/v4"
)

// CompressData comprime dados usando o algoritmo LZ4
// Retorna os dados comprimidos ou um erro se a compressão falhar
func CompressData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	// Criar buffer para armazenar dados comprimidos
	var buf bytes.Buffer
	
	// Criar writer LZ4 com configuração para melhor compressão
	zw := lz4.NewWriter(&buf)
	
	// Configurar compressão para melhor compressão
	zw.Apply(lz4.ChecksumOption(true))
	zw.Apply(lz4.CompressionLevelOption(lz4.Level9)) // Melhor compressão
	
	// Escrever dados no compressor
	if _, err := zw.Write(data); err != nil {
		return nil, err
	}
	
	// Fechar writer para garantir que todos os dados foram comprimidos
	if err := zw.Close(); err != nil {
		return nil, err
	}
	
	// Retornar dados comprimidos
	return buf.Bytes(), nil
}

// DecompressData descomprime dados comprimidos com LZ4
// Retorna os dados descomprimidos ou um erro se a descompressão falhar
func DecompressData(compressedData []byte) ([]byte, error) {
	if len(compressedData) == 0 {
		return compressedData, nil
	}

	// Criar reader para dados comprimidos
	r := bytes.NewReader(compressedData)
	
	// Criar reader LZ4
	zr := lz4.NewReader(r)
	
	// Ler dados descomprimidos
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, zr); err != nil {
		return nil, err
	}
	
	// Retornar dados descomprimidos
	return buf.Bytes(), nil
}

// ShouldCompress determina se um tipo de dados deve ser comprimido
// Baseado no tipo MIME ou extensão de arquivo
func ShouldCompress(mimeType string) bool {
	// Tipos que já são comprimidos e não se beneficiam de compressão adicional
	alreadyCompressedTypes := map[string]bool{
		"image/jpeg":      true,
		"image/png":       true,
		"image/gif":       true,
		"image/webp":      true,
		"audio/mp3":       true,
		"audio/ogg":       true,
		"video/mp4":       true,
		"video/webm":      true,
		"application/zip": true,
		"application/gzip": true,
		"application/x-rar-compressed": true,
	}
	
	return !alreadyCompressedTypes[mimeType]
}

// CompressIfNeeded comprime dados apenas se o tipo de conteúdo se beneficiar de compressão
// Retorna os dados (comprimidos ou não) e um booleano indicando se foram comprimidos
func CompressIfNeeded(data []byte, mimeType string) ([]byte, bool, error) {
	if !ShouldCompress(mimeType) || len(data) < 100 {
		// Não comprimir se o tipo já é comprimido ou se os dados são muito pequenos
		return data, false, nil
	}
	
	compressed, err := CompressData(data)
	if err != nil {
		return nil, false, err
	}
	
	// Verificar se a compressão realmente reduziu o tamanho
	if len(compressed) >= len(data) {
		// Compressão não foi eficiente, retornar dados originais
		return data, false, nil
	}
	
	return compressed, true, nil
}
