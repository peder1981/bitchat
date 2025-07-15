package protocol

import (
	"bytes"
	"encoding/binary"
)

// PacketDataForSignature gera os dados a serem assinados para um pacote
// Inclui todos os campos relevantes exceto a própria assinatura
func PacketDataForSignature(packet *BitchatPacket) []byte {
	// Criar buffer para armazenar os dados
	buf := new(bytes.Buffer)
	
	// Adicionar todos os campos relevantes na ordem correta
	buf.WriteByte(packet.Version)
	buf.WriteByte(byte(packet.Type))
	buf.Write(packet.SenderID)
	buf.Write(packet.RecipientID)
	binary.Write(buf, binary.BigEndian, packet.Timestamp)
	buf.WriteByte(packet.TTL)
	buf.Write(packet.Payload)
	
	return buf.Bytes()
}

// BytesToMessage converte bytes para uma mensagem
// Alias para MessageFromBytes para compatibilidade com os testes
func BytesToMessage(data []byte) (*Message, error) {
	return MessageFromBytes(data)
}

// MessageToPacket converte uma Message para um BitchatPacket
func MessageToPacket(message *Message) *BitchatPacket {
	return &BitchatPacket{
		Version:    CurrentVersion,
		Type:       message.Type,
		SenderID:   message.SenderID,
		RecipientID: message.RecipientID,
		Timestamp:  message.Timestamp,
		Payload:    message.Content,
		TTL:        7, // Valor padrão
		ID:         message.ID(),
	}
}

// PacketToMessage converte um BitchatPacket para uma Message
func PacketToMessage(packet *BitchatPacket) *Message {
	return &Message{
		MessageID:   packet.ID,
		Type:        packet.Type,
		Content:     packet.Payload,
		SenderID:    packet.SenderID,
		RecipientID: packet.RecipientID,
		Timestamp:   packet.Timestamp,
		Compressed:  false,
		Encrypted:   false,
		Nonce:       nil,
		Channel:     "",
	}
}
