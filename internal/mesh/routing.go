package mesh

import (
	"sync"

	"github.com/permissionlesstech/bitchat/internal/protocol"
)

// RoutingConfig contém configurações para o roteador de mensagens
type RoutingConfig struct {
	// Número máximo de saltos (TTL) para mensagens
	MaxHops int
	
	// Se verdadeiro, permite relay de mensagens para outros peers
	AllowRelay bool
	
	// Se verdadeiro, permite mensagens de broadcast
	AllowBroadcast bool
	
	// Lista de IDs de peers bloqueados
	BlockedPeers []string
}

// DefaultRoutingConfig retorna uma configuração padrão para o roteador
func DefaultRoutingConfig() *RoutingConfig {
	return &RoutingConfig{
		MaxHops:        3,
		AllowRelay:     true,
		AllowBroadcast: true,
		BlockedPeers:   []string{},
	}
}

// Router gerencia o roteamento de mensagens na rede mesh
type Router struct {
	// Configuração do roteador
	config *RoutingConfig
	
	// Mutex para proteção de dados
	mutex sync.RWMutex
	
	// Função para enviar mensagens para um peer específico
	sendFunc func(packet *protocol.BitchatPacket, targetPeerID string) error
	
	// Mapa de peers bloqueados para acesso rápido
	blockedPeersMap map[string]bool
}

// NewRouter cria um novo roteador de mensagens
func NewRouter(config *RoutingConfig, sendFunc func(packet *protocol.BitchatPacket, targetPeerID string) error) *Router {
	if config == nil {
		config = DefaultRoutingConfig()
	}
	
	// Converter lista de peers bloqueados para mapa
	blockedMap := make(map[string]bool)
	for _, peerID := range config.BlockedPeers {
		blockedMap[peerID] = true
	}
	
	return &Router{
		config:         config,
		sendFunc:       sendFunc,
		blockedPeersMap: blockedMap,
	}
}

// RoutePacket roteia um pacote para o destinatário apropriado
func (r *Router) RoutePacket(packet *protocol.BitchatPacket, knownPeers []string) error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	// Verificar se o remetente está bloqueado
	if r.isBlocked(string(packet.SenderID)) {
		return nil // Silenciosamente ignorar pacotes de peers bloqueados
	}
	
	// Verificar TTL
	if packet.TTL <= 0 {
		return nil // Pacote expirou, não rotear
	}
	
	// Decrementar TTL para evitar loops infinitos
	packet.TTL--
	
	// Determinar destinatários
	if packet.RecipientID == nil || len(packet.RecipientID) == 0 {
		// Broadcast para todos os peers conhecidos
		if !r.config.AllowBroadcast {
			return nil // Broadcast não permitido
		}
		
		for _, peerID := range knownPeers {
			if !r.isBlocked(peerID) {
				r.sendFunc(packet, peerID)
			}
		}
	} else {
		// Mensagem direcionada
		recipientID := string(packet.RecipientID)
		
		// Verificar se o destinatário está bloqueado
		if r.isBlocked(recipientID) {
			return nil
		}
		
		// Enviar para o destinatário específico
		return r.sendFunc(packet, recipientID)
	}
	
	return nil
}

// BlockPeer adiciona um peer à lista de bloqueados
func (r *Router) BlockPeer(peerID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	r.blockedPeersMap[peerID] = true
}

// UnblockPeer remove um peer da lista de bloqueados
func (r *Router) UnblockPeer(peerID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	delete(r.blockedPeersMap, peerID)
}

// IsBlocked verifica se um peer está bloqueado
func (r *Router) IsBlocked(peerID string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	return r.isBlocked(peerID)
}

// isBlocked (versão interna sem lock) verifica se um peer está bloqueado
func (r *Router) isBlocked(peerID string) bool {
	return r.blockedPeersMap[peerID]
}

// GetBlockedPeers retorna a lista de peers bloqueados
func (r *Router) GetBlockedPeers() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	result := make([]string, 0, len(r.blockedPeersMap))
	for peerID := range r.blockedPeersMap {
		result = append(result, peerID)
	}
	
	return result
}

// UpdateConfig atualiza a configuração do roteador
func (r *Router) UpdateConfig(config *RoutingConfig) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	r.config = config
	
	// Atualizar mapa de peers bloqueados
	r.blockedPeersMap = make(map[string]bool)
	for _, peerID := range config.BlockedPeers {
		r.blockedPeersMap[peerID] = true
	}
}
