package mesh

import (
	"sync"
	"time"

	"github.com/permissionlesstech/bitchat/internal/protocol"
	"github.com/permissionlesstech/bitchat/pkg/utils"
)

// MessageRouter gerencia o roteamento e deduplicação de mensagens na rede mesh
type MessageRouter struct {
	// Cache de mensagens já processadas para deduplicação
	processedMessages *utils.ExpiringSet
	
	// Tabela de roteamento: peerID -> nextHop
	routingTable      map[string]string
	
	// Métricas de roteamento: peerID -> qualidade da conexão (0-100)
	routingMetrics    map[string]int
	
	// Mutex para proteger a tabela de roteamento
	routingMutex      sync.RWMutex
	
	// TTL padrão para mensagens
	defaultTTL        uint8
	
	// Tempo máximo de cache para deduplicação
	dedupeTime        time.Duration
}

// NewMessageRouter cria um novo roteador de mensagens
func NewMessageRouter() *MessageRouter {
	return &MessageRouter{
		// Cache de 10 minutos com limpeza a cada minuto
		processedMessages: utils.NewExpiringSet(10*time.Minute, 1*time.Minute),
		routingTable:      make(map[string]string),
		routingMetrics:    make(map[string]int),
		defaultTTL:        5,  // TTL padrão: 5 hops
		dedupeTime:        10*time.Minute,
	}
}

// NewRouter cria um novo roteador de mensagens com configuração
func NewRouter(config *RoutingConfig) *MessageRouter {
	var dedupeTime time.Duration
	if config != nil && config.DeduplicationTTL > 0 {
		dedupeTime = config.DeduplicationTTL
	} else {
		dedupeTime = 10 * time.Minute
	}
	
	var defaultTTL uint8
	if config != nil && config.MaxTTL > 0 {
		defaultTTL = config.MaxTTL
	} else {
		defaultTTL = 5
	}
	
	return &MessageRouter{
		processedMessages: utils.NewExpiringSet(dedupeTime, 1*time.Minute),
		routingTable:      make(map[string]string),
		routingMetrics:    make(map[string]int),
		defaultTTL:        defaultTTL,
		dedupeTime:        dedupeTime,
	}
}

// ShouldProcess verifica se uma mensagem deve ser processada ou descartada
// Retorna true se a mensagem deve ser processada, false se é duplicada ou expirada
func (mr *MessageRouter) ShouldProcess(packet *protocol.BitchatPacket) bool {
	// Verificar TTL
	if packet.TTL == 0 {
		return false
	}
	
	// Verificar deduplicação
	messageID := packet.ID
	return mr.processedMessages.Add(messageID)
}

// MarkProcessed marca uma mensagem como processada para evitar duplicação
func (mr *MessageRouter) MarkProcessed(packet *protocol.BitchatPacket) {
	messageID := packet.ID
	mr.processedMessages.Add(messageID)
}

// DecreaseAndCheckTTL diminui o TTL de um pacote e verifica se ainda é válido
// Retorna true se o pacote ainda é válido (TTL > 0), false se expirou
func (mr *MessageRouter) DecreaseAndCheckTTL(packet *protocol.BitchatPacket) bool {
	if packet.TTL <= 1 {
		return false
	}
	
	packet.TTL--
	return true
}

// SetDefaultTTL define o TTL padrão para novas mensagens
func (mr *MessageRouter) SetDefaultTTL(ttl uint8) {
	mr.defaultTTL = ttl
}

// GetDefaultTTL retorna o TTL padrão atual
func (mr *MessageRouter) GetDefaultTTL() uint8 {
	return mr.defaultTTL
}

// SetDedupeTime define o tempo de cache para deduplicação
func (mr *MessageRouter) SetDedupeTime(duration time.Duration) {
	mr.dedupeTime = duration
	mr.processedMessages.SetTTL(duration)
}

// UpdateRoutingInfo atualiza a tabela de roteamento com informações de um peer
func (mr *MessageRouter) UpdateRoutingInfo(peerID string, nextHop string, metric int) {
	mr.routingMutex.Lock()
	defer mr.routingMutex.Unlock()
	
	// Se o nextHop for vazio, é uma conexão direta
	if nextHop == "" {
		nextHop = peerID
	}
	
	// Atualizar tabela de roteamento
	currentMetric, hasRoute := mr.routingMetrics[peerID]
	
	// Atualizar apenas se não temos rota ou a nova rota é melhor
	if !hasRoute || metric > currentMetric {
		mr.routingTable[peerID] = nextHop
		mr.routingMetrics[peerID] = metric
	}
}

// GetNextHop determina o próximo hop para um destinatário
// Retorna o ID do próximo peer e um booleano indicando se o destinatário é alcançável
func (mr *MessageRouter) GetNextHop(recipientID string) (string, bool) {
	mr.routingMutex.RLock()
	defer mr.routingMutex.RUnlock()
	
	nextHop, exists := mr.routingTable[recipientID]
	return nextHop, exists
}

// RemovePeer remove um peer da tabela de roteamento
func (mr *MessageRouter) RemovePeer(peerID string) {
	mr.routingMutex.Lock()
	defer mr.routingMutex.Unlock()
	
	// Remover peer da tabela de roteamento
	delete(mr.routingTable, peerID)
	delete(mr.routingMetrics, peerID)
	
	// Remover rotas que passam por este peer
	for dest, hop := range mr.routingTable {
		if hop == peerID {
			delete(mr.routingTable, dest)
			delete(mr.routingMetrics, dest)
		}
	}
}

// GetAllPeers retorna todos os peers conhecidos (direta ou indiretamente)
func (mr *MessageRouter) GetAllPeers() []string {
	mr.routingMutex.RLock()
	defer mr.routingMutex.RUnlock()
	
	peers := make([]string, 0, len(mr.routingTable))
	for peer := range mr.routingTable {
		peers = append(peers, peer)
	}
	
	return peers
}

// GetDirectPeers retorna apenas os peers diretamente conectados
func (mr *MessageRouter) GetDirectPeers() []string {
	mr.routingMutex.RLock()
	defer mr.routingMutex.RUnlock()
	
	directPeers := make([]string, 0)
	for peer, hop := range mr.routingTable {
		if peer == hop {
			directPeers = append(directPeers, peer)
		}
	}
	
	return directPeers
}

// PrepareOutgoingPacket prepara um pacote para envio
// Define o TTL e outros parâmetros necessários
func (mr *MessageRouter) PrepareOutgoingPacket(packet *protocol.BitchatPacket) {
	// Definir TTL se não estiver definido
	if packet.TTL == 0 {
		packet.TTL = mr.defaultTTL
	}
}

// Clear limpa todas as informações de roteamento
func (mr *MessageRouter) Clear() {
	mr.routingMutex.Lock()
	defer mr.routingMutex.Unlock()
	
	mr.routingTable = make(map[string]string)
	mr.routingMetrics = make(map[string]int)
	mr.processedMessages.Clear()
}

// Stop interrompe o roteador e libera recursos
func (mr *MessageRouter) Stop() {
	mr.processedMessages.Stop()
}
