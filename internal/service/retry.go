package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/permissionlesstech/bitchat/internal/protocol"
)

// RetryConfig define as configurações para o serviço de retry
type RetryConfig struct {
	// Número máximo de tentativas
	MaxRetries int
	
	// Intervalo inicial entre tentativas
	InitialBackoff time.Duration
	
	// Fator de crescimento do backoff
	BackoffFactor float64
	
	// Intervalo máximo entre tentativas
	MaxBackoff time.Duration
	
	// Tempo máximo total para tentar entregar uma mensagem
	MaxRetryTime time.Duration
}

// DefaultRetryConfig retorna uma configuração padrão para o serviço de retry
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 5 * time.Second,
		BackoffFactor:  1.5,
		MaxBackoff:     2 * time.Minute,
		MaxRetryTime:   30 * time.Minute,
	}
}

// RetryItem representa uma mensagem em retry
type RetryItem struct {
	// Pacote a ser reenviado
	Packet *protocol.BitchatPacket
	
	// ID do destinatário (pode ser diferente do recipientID do pacote em caso de relay)
	TargetPeerID string
	
	// Número de tentativas já realizadas
	Attempts int
	
	// Timestamp da primeira tentativa
	FirstAttempt time.Time
	
	// Timestamp da próxima tentativa
	NextAttempt time.Time
	
	// Callback a ser chamado quando a mensagem for entregue ou falhar
	OnComplete func(messageID string, success bool, info *protocol.DeliveryInfo)
}

// RetryService gerencia o retry de mensagens não entregues
type RetryService struct {
	// Configuração do serviço
	config *RetryConfig
	
	// Mapa de mensagens em retry: messageID -> RetryItem
	retryItems map[string]*RetryItem
	
	// Mutex para proteger o mapa de retry
	mutex sync.RWMutex
	
	// Canal para sinalizar parada
	stopChan chan struct{}
	
	// WaitGroup para esperar goroutines
	wg sync.WaitGroup
	
	// Função de callback para enviar pacotes
	sendPacketFunc func(packet *protocol.BitchatPacket, targetPeerID string) error
}

// NewRetryService cria um novo serviço de retry
func NewRetryService(config *RetryConfig, sendFunc ...func(packet *protocol.BitchatPacket, targetPeerID string) error) *RetryService {
	if config == nil {
		config = DefaultRetryConfig()
	}
	
	var sendPacketFunc func(packet *protocol.BitchatPacket, targetPeerID string) error
	if len(sendFunc) > 0 {
		sendPacketFunc = sendFunc[0]
	} else {
		// Função padrão que não faz nada (para compatibilidade com testes)
		sendPacketFunc = func(packet *protocol.BitchatPacket, targetPeerID string) error {
			return nil
		}
	}

	return &RetryService{
		config:        config,
		retryItems:    make(map[string]*RetryItem),
		stopChan:      make(chan struct{}),
		sendPacketFunc: sendPacketFunc,
	}
}

// Start inicia o serviço de retry
func (rs *RetryService) Start() {
	rs.wg.Add(1)
	go rs.retryLoop()
}

// Stop interrompe o serviço de retry
func (rs *RetryService) Stop() {
	close(rs.stopChan)
	rs.wg.Wait()
}

// AddRetryPacket adiciona uma mensagem para retry
func (rs *RetryService) AddRetryPacket(packet *protocol.BitchatPacket, targetPeerID string, onComplete func(messageID string, success bool, info *protocol.DeliveryInfo)) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	
	messageID := packet.ID
	
	// Verificar se já existe um retry para esta mensagem
	if _, exists := rs.retryItems[messageID]; exists {
		return
	}
	
	now := time.Now()
	
	// Criar novo item de retry
	item := &RetryItem{
		Packet:       packet,
		TargetPeerID: targetPeerID,
		Attempts:     1, // Já consideramos a primeira tentativa
		FirstAttempt: now,
		NextAttempt:  now.Add(rs.config.InitialBackoff),
		OnComplete:   onComplete,
	}
	
	rs.retryItems[messageID] = item
	
	if rs.config.MaxRetryTime > 0 {
		// Agendar expiração final
		go func(id string) {
			select {
			case <-time.After(rs.config.MaxRetryTime):
				rs.handleFailedDelivery(id)
			case <-rs.stopChan:
				return
			}
		}(messageID)
	}
}

// MarkDelivered marca uma mensagem como entregue
func (rs *RetryService) MarkDelivered(messageID string) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	
	if item, exists := rs.retryItems[messageID]; exists {
		// Chamar callback de sucesso
		if item.OnComplete != nil {
			info := &protocol.DeliveryInfo{
				Status:    protocol.DeliveryStatusDelivered,
				Timestamp: uint64(time.Now().UnixMilli()),
				Attempts:  item.Attempts,
			}
			
			item.OnComplete(messageID, true, info)
		}
		
		// Remover do mapa de retry
		delete(rs.retryItems, messageID)
	}
}

// GetPendingCount retorna o número de mensagens pendentes
func (rs *RetryService) GetPendingCount() int {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()
	
	return len(rs.retryItems)
}

// GetPendingMessages retorna as mensagens pendentes
func (rs *RetryService) GetPendingMessages() []*protocol.BitchatPacket {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()
	
	result := make([]*protocol.BitchatPacket, 0, len(rs.retryItems))
	for _, item := range rs.retryItems {
		result = append(result, item.Packet)
	}
	
	return result
}

// retryLoop é a goroutine principal que gerencia os retries
func (rs *RetryService) retryLoop() {
	defer rs.wg.Done()
	
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			rs.processRetries()
		case <-rs.stopChan:
			return
		}
	}
}

// processRetries processa as mensagens que precisam ser reenviadas
func (rs *RetryService) processRetries() {
	now := time.Now()
	var itemsToRetry []*RetryItem
	var itemsToRemove []string
	
	// Coletar itens que precisam ser reenviados
	rs.mutex.RLock()
	for id, item := range rs.retryItems {
		if now.After(item.NextAttempt) {
			if item.Attempts >= rs.config.MaxRetries {
				itemsToRemove = append(itemsToRemove, id)
			} else {
				itemsToRetry = append(itemsToRetry, item)
			}
		}
	}
	rs.mutex.RUnlock()
	
	// Reenviar mensagens
	for _, item := range itemsToRetry {
		rs.retryMessage(item)
	}
	
	// Remover mensagens que excederam o número máximo de tentativas
	for _, id := range itemsToRemove {
		rs.handleFailedDelivery(id)
	}
}

// retryMessage reenvia uma mensagem
func (rs *RetryService) retryMessage(item *RetryItem) {
	// Incrementar contador de tentativas
	rs.mutex.Lock()
	item.Attempts++
	
	// Calcular próximo backoff com exponential backoff
	backoff := time.Duration(float64(rs.config.InitialBackoff) * 
		float64(item.Attempts-1) * rs.config.BackoffFactor)
	
	// Limitar ao backoff máximo
	if backoff > rs.config.MaxBackoff {
		backoff = rs.config.MaxBackoff
	}
	
	// Definir próxima tentativa
	item.NextAttempt = time.Now().Add(backoff)
	rs.mutex.Unlock()
	
	// Tentar reenviar a mensagem
	err := rs.sendPacketFunc(item.Packet, item.TargetPeerID)
	if err != nil {
		fmt.Printf("Erro ao reenviar mensagem %s (tentativa %d): %v\n", 
			item.Packet.ID, item.Attempts, err)
	}
}

// handleFailedDelivery lida com mensagens que falharam todas as tentativas
func (rs *RetryService) handleFailedDelivery(messageID string) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	
	if item, exists := rs.retryItems[messageID]; exists {
		// Chamar callback de falha
		if item.OnComplete != nil {
			info := &protocol.DeliveryInfo{
				Status:     protocol.DeliveryStatusFailed,
				Timestamp:  uint64(time.Now().UnixMilli()),
				Attempts:   item.Attempts,
				Error:      "Número máximo de tentativas excedido",
				FailReason: "Número máximo de tentativas excedido",
			}
			
			item.OnComplete(messageID, false, info)
		}
		
		// Remover do mapa de retry
		delete(rs.retryItems, messageID)
	}
}

// ClearRetries limpa todas as mensagens em retry
func (rs *RetryService) ClearRetries() {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	
	rs.retryItems = make(map[string]*RetryItem)
}
