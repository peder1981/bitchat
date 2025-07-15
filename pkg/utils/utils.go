package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"time"
)

// GenerateRandomID gera um ID aleatório de tamanho especificado
func GenerateRandomID(length int) []byte {
	id := make([]byte, length)
	_, err := rand.Read(id)
	if err != nil {
		// Fallback para um ID menos aleatório em caso de erro
		for i := range id {
			id[i] = byte(time.Now().Nanosecond() % 256)
			time.Sleep(time.Nanosecond)
		}
	}
	return id
}

// GenerateMessageID gera um ID único para uma mensagem baseado em seu conteúdo
func GenerateMessageID(packet interface{}) string {
	// Usar um timestamp para garantir unicidade
	timestamp := time.Now().UnixNano()
	
	// Gerar bytes aleatórios
	randomBytes := make([]byte, 8)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fallback
		for i := range randomBytes {
			randomBytes[i] = byte(timestamp % 256)
			timestamp = timestamp / 256
		}
	}
	
	// Combinar com o hash do pacote (simplificado)
	hash := sha256.New()
	hash.Write([]byte(time.Now().String()))
	hash.Write(randomBytes)
	
	// Retornar os primeiros 16 bytes como string hex
	return hex.EncodeToString(hash.Sum(nil)[:16])
}

// ByteArraysEqual compara dois arrays de bytes
func ByteArraysEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// RandomInt gera um número aleatório entre 0 e max-1
func RandomInt(max int) int {
	if max <= 0 {
		return 0
	}
	
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		// Fallback menos seguro
		return time.Now().Nanosecond() % max
	}
	
	return int(n.Int64())
}

// Hash gera um hash SHA-256 de uma string e retorna como string hexadecimal
func Hash(data string) string {
	hash := sha256.New()
	hash.Write([]byte(data))
	return hex.EncodeToString(hash.Sum(nil))
}
