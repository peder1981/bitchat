package service

import (
	"github.com/permissionlesstech/bitchat/internal/protocol"
	"time"
)

// AddRetry adiciona uma mensagem para retry (versão simplificada para compatibilidade com testes)
// Esta versão aceita apenas o ID da mensagem e uma função de callback sem parâmetros
func (rs *RetryService) AddRetry(messageID string, onComplete func()) {
	// Criar um pacote fictício para compatibilidade
	packet := &protocol.BitchatPacket{
		ID: messageID,
	}

	// Adaptar o callback para o formato esperado pelo RetryService
	callback := func(msgID string, success bool, info *protocol.DeliveryInfo) {
		if onComplete != nil {
			onComplete()
		}
	}

	// Chamar a implementação real com os parâmetros adaptados
	rs.AddRetryPacket(packet, "default", callback)
}

// DeliveryInfo é uma estrutura de compatibilidade para os testes de integração
type DeliveryInfo struct {
	PacketID    string
	RecipientID string
	MaxRetries  int
	NextRetry   time.Time
}

// AddRetryCompat adiciona uma mensagem para retry (compatível com os testes de integração)
// Esta versão aceita DeliveryInfo e BitchatPacket
func (rs *RetryService) AddRetryCompat(info *DeliveryInfo, packet *protocol.BitchatPacket) {
	// Adaptar o callback para o formato esperado pelo RetryService
	callback := func(msgID string, success bool, deliveryInfo *protocol.DeliveryInfo) {
		// Nada a fazer no teste
	}

	// Chamar a implementação real com os parâmetros adaptados
	rs.AddRetryPacket(packet, info.RecipientID, callback)
}
