package mesh

import "time"

// RoutingConfig contém configurações para o serviço de roteamento
type RoutingConfig struct {
	MaxTTL            uint8         // Valor máximo de TTL para pacotes
	DeduplicationTTL  time.Duration // Tempo de vida para deduplicação de mensagens
	PeerTTL           time.Duration // Tempo de vida para peers na tabela de roteamento
	MaxPeers          int           // Número máximo de peers na tabela de roteamento
}
