package crypto

import (
	"encoding/hex"
)

// StoreEphemeralKeyCompat é um wrapper para StoreEphemeralKey que aceita chaves em formato string (hex)
// para compatibilidade com os testes de integração
func (es *EncryptionService) StoreEphemeralKeyCompat(peerID string, keyHex string) error {
	// Converter a chave de string hex para bytes
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return err
	}
	
	// Usar o método existente
	return es.StoreEphemeralKey(peerID, key)
}
