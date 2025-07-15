package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

// Erros relacionados ao protocolo binário
var (
	ErrInvalidPacket = errors.New("pacote inválido ou corrompido")
	ErrBufferTooSmall = errors.New("buffer muito pequeno para decodificar o pacote")
)

// Encode serializa um BitchatPacket em um formato binário eficiente
func Encode(packet *BitchatPacket) ([]byte, error) {
	// Calcular o tamanho total do buffer
	size := 1 + 1 + 1 + len(packet.SenderID) + 1
	if packet.RecipientID != nil {
		size += len(packet.RecipientID)
	}
	size += 8 + 4 + len(packet.Payload)
	if packet.Signature != nil {
		size += len(packet.Signature)
	}
	size += 1 // TTL

	// Criar buffer
	buf := bytes.NewBuffer(make([]byte, 0, size))

	// Escrever versão
	if err := buf.WriteByte(packet.Version); err != nil {
		return nil, err
	}

	// Escrever tipo
	if err := buf.WriteByte(byte(packet.Type)); err != nil {
		return nil, err
	}

	// Escrever tamanho e dados do SenderID
	if err := buf.WriteByte(byte(len(packet.SenderID))); err != nil {
		return nil, err
	}
	if _, err := buf.Write(packet.SenderID); err != nil {
		return nil, err
	}

	// Escrever tamanho e dados do RecipientID (se presente)
	if packet.RecipientID != nil {
		if err := buf.WriteByte(byte(len(packet.RecipientID))); err != nil {
			return nil, err
		}
		if _, err := buf.Write(packet.RecipientID); err != nil {
			return nil, err
		}
	} else {
		if err := buf.WriteByte(0); err != nil {
			return nil, err
		}
	}

	// Escrever timestamp
	if err := binary.Write(buf, binary.BigEndian, packet.Timestamp); err != nil {
		return nil, err
	}

	// Escrever tamanho e dados do Payload
	if err := binary.Write(buf, binary.BigEndian, uint32(len(packet.Payload))); err != nil {
		return nil, err
	}
	if _, err := buf.Write(packet.Payload); err != nil {
		return nil, err
	}

	// Escrever tamanho e dados da Signature (se presente)
	if packet.Signature != nil {
		if err := buf.WriteByte(byte(len(packet.Signature))); err != nil {
			return nil, err
		}
		if _, err := buf.Write(packet.Signature); err != nil {
			return nil, err
		}
	} else {
		if err := buf.WriteByte(0); err != nil {
			return nil, err
		}
	}

	// Escrever TTL
	if err := buf.WriteByte(packet.TTL); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Decode deserializa um BitchatPacket a partir de dados binários
func Decode(data []byte) (*BitchatPacket, error) {
	if len(data) < 13 { // Tamanho mínimo para um pacote válido
		return nil, ErrBufferTooSmall
	}

	buf := bytes.NewBuffer(data)
	packet := &BitchatPacket{}

	// Ler versão
	version, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	packet.Version = version

	// Ler tipo
	msgType, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	packet.Type = MessageType(msgType)

	// Ler SenderID
	senderIDLen, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	if senderIDLen > 0 {
		packet.SenderID = make([]byte, senderIDLen)
		if _, err := io.ReadFull(buf, packet.SenderID); err != nil {
			return nil, err
		}
	}

	// Ler RecipientID
	recipientIDLen, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	if recipientIDLen > 0 {
		packet.RecipientID = make([]byte, recipientIDLen)
		if _, err := io.ReadFull(buf, packet.RecipientID); err != nil {
			return nil, err
		}
	}

	// Ler timestamp
	var timestamp uint64
	if err := binary.Read(buf, binary.BigEndian, &timestamp); err != nil {
		return nil, err
	}
	packet.Timestamp = timestamp

	// Ler Payload
	var payloadLen uint32
	if err := binary.Read(buf, binary.BigEndian, &payloadLen); err != nil {
		return nil, err
	}
	if payloadLen > 0 {
		packet.Payload = make([]byte, payloadLen)
		if _, err := io.ReadFull(buf, packet.Payload); err != nil {
			return nil, err
		}
	}

	// Ler Signature
	signatureLen, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	if signatureLen > 0 {
		packet.Signature = make([]byte, signatureLen)
		if _, err := io.ReadFull(buf, packet.Signature); err != nil {
			return nil, err
		}
	}

	// Ler TTL
	ttl, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	packet.TTL = ttl

	return packet, nil
}

// MessagePadding implementa utilitários de padding para privacidade
type MessagePadding struct{}

// Tamanhos padrão de blocos para padding
var blockSizes = []int{256, 512, 1024, 2048}

// Pad adiciona padding estilo PKCS#7 para atingir o tamanho alvo
func (mp *MessagePadding) Pad(data []byte, targetSize int) []byte {
	if len(data) >= targetSize {
		return data
	}

	paddingNeeded := targetSize - len(data)
	if paddingNeeded > 255 {
		return data // PKCS#7 suporta apenas padding até 255 bytes
	}

	padded := make([]byte, len(data)+paddingNeeded)
	copy(padded, data)

	// Preencher com bytes aleatórios
	for i := len(data); i < len(padded)-1; i++ {
		padded[i] = byte(i % 256) // Simplificado para determinismo, em produção usar crypto/rand
	}
	padded[len(padded)-1] = byte(paddingNeeded)

	return padded
}

// Unpad remove o padding dos dados
func (mp *MessagePadding) Unpad(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	paddingLength := int(data[len(data)-1])
	if paddingLength <= 0 || paddingLength > len(data) {
		return data
	}

	return data[:len(data)-paddingLength]
}

// OptimalBlockSize encontra o tamanho de bloco ideal para os dados
func (mp *MessagePadding) OptimalBlockSize(dataSize int) int {
	// Considerar overhead de criptografia (~16 bytes para tag AES-GCM)
	totalSize := dataSize + 16

	// Encontrar o menor bloco que cabe
	for _, blockSize := range blockSizes {
		if totalSize <= blockSize {
			return blockSize
		}
	}

	// Para mensagens muito grandes, usar o tamanho original
	// (será fragmentado de qualquer forma)
	return dataSize
}
