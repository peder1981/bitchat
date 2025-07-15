package protocol

import (
	"encoding/json"
)

// Message representa uma mensagem no formato usado pelos testes de integração
type Message struct {
	MessageID    string `json:"id"`
	Type        MessageType
	Content     []byte
	SenderID    []byte
	RecipientID []byte
	Timestamp   uint64
	Compressed  bool
	Encrypted   bool
	Nonce       []byte
	Channel     string
}

// MessageToBytes serializa uma mensagem para bytes
func MessageToBytes(message *Message) []byte {
	data, err := json.Marshal(message)
	if err != nil {
		return nil
	}
	return data
}

// MessageFromBytes deserializa bytes para uma mensagem
func MessageFromBytes(data []byte) (*Message, error) {
	var message Message
	err := json.Unmarshal(data, &message)
	if err != nil {
		return nil, err
	}
	return &message, nil
}

// ID retorna o ID da mensagem
func (m *Message) ID() string {
	return m.MessageID
}
