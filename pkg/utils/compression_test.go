package utils

import (
	"bytes"
	"testing"
)

func TestCompressDecompress(t *testing.T) {
	testCases := []struct {
		name     string
		data     []byte
		mimeType string
		compress bool
	}{
		{
			name:     "Texto simples",
			data:     []byte("Este é um texto simples que deve comprimir bem devido à repetição de caracteres."),
			mimeType: "text/plain",
			compress: true,
		},
		{
			name:     "Dados JSON",
			data:     []byte(`{"name":"teste","description":"Este é um teste de compressão JSON","items":["item1","item2","item3"],"numbers":[1,2,3,4,5]}`),
			mimeType: "application/json",
			compress: true,
		},
		{
			name:     "Dados binários aleatórios",
			data:     generateRandomBytes(1000),
			mimeType: "application/octet-stream",
			compress: true,
		},
		{
			name:     "Imagem JPEG (já comprimida)",
			data:     generateFakeJPEG(500),
			mimeType: "image/jpeg",
			compress: false,
		},
		{
			name:     "Dados muito pequenos",
			data:     []byte("abc"),
			mimeType: "text/plain",
			compress: false, // Muito pequeno para comprimir eficientemente
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Testar compressão
			compressed, err := CompressData(tc.data)
			if err != nil {
				t.Fatalf("Erro ao comprimir dados: %v", err)
			}

			// Testar descompressão
			decompressed, err := DecompressData(compressed)
			if err != nil {
				t.Fatalf("Erro ao descomprimir dados: %v", err)
			}

			// Verificar se os dados descomprimidos são iguais aos originais
			if !bytes.Equal(tc.data, decompressed) {
				t.Errorf("Dados descomprimidos não correspondem aos originais")
			}

			// Testar ShouldCompress
			if ShouldCompress(tc.mimeType) != tc.compress {
				t.Errorf("ShouldCompress(%s) = %v, esperado %v", tc.mimeType, ShouldCompress(tc.mimeType), tc.compress)
			}

			// Testar CompressIfNeeded
			result, compressed, err := CompressIfNeeded(tc.data, tc.mimeType)
			if err != nil {
				t.Fatalf("Erro em CompressIfNeeded: %v", err)
			}

			// Se não deveria comprimir, o resultado deve ser igual aos dados originais
			if !tc.compress && !bytes.Equal(result, tc.data) {
				t.Errorf("CompressIfNeeded deveria retornar os dados originais")
			}

			// Se comprimiu, deve ser possível descomprimir
			if compressed {
				decompressed, err := DecompressData(result)
				if err != nil {
					t.Fatalf("Erro ao descomprimir resultado de CompressIfNeeded: %v", err)
				}
				if !bytes.Equal(decompressed, tc.data) {
					t.Errorf("Dados descomprimidos de CompressIfNeeded não correspondem aos originais")
				}
			}
		})
	}
}

// Funções auxiliares para gerar dados de teste

func generateRandomBytes(size int) []byte {
	result := make([]byte, size)
	for i := 0; i < size; i++ {
		result[i] = byte(i % 256)
	}
	return result
}

func generateFakeJPEG(size int) []byte {
	// Cabeçalho JPEG simplificado
	header := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	
	// Gerar "dados" da imagem
	data := make([]byte, size-len(header)-2)
	for i := 0; i < len(data); i++ {
		data[i] = byte(i % 256)
	}
	
	// Rodapé JPEG
	footer := []byte{0xFF, 0xD9}
	
	// Combinar tudo
	result := append(header, data...)
	result = append(result, footer...)
	
	return result
}
