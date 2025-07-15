package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/permissionlesstech/bitchat/internal/protocol"
)

// Erros do MessageStore
var (
	ErrMessageNotFound = errors.New("mensagem não encontrada")
	ErrInvalidChannel  = errors.New("canal inválido")
	ErrInvalidPeer     = errors.New("peer inválido")
)

// MessageStoreConfig contém configurações para o serviço de armazenamento
type MessageStoreConfig struct {
	// Diretório para armazenamento persistente
	StoreDir string
	// Período de retenção de mensagens (0 = sem expiração)
	RetentionPeriod time.Duration
	// Número máximo de mensagens por peer/canal
	MaxMessagesPerPeer int
}

// MessageStore gerencia o armazenamento persistente de mensagens
type MessageStore struct {
	config          *MessageStoreConfig
	pendingMessages map[string]*protocol.Message         // Mensagens pendentes por ID
	peerMessages    map[string][]*protocol.Message       // Mensagens por peer
	channelMessages map[string][]*protocol.Message       // Mensagens por canal
	mutex           sync.RWMutex
	cleanupTicker   *time.Ticker
	stopChan        chan struct{}
}

// NewMessageStore cria um novo serviço de armazenamento de mensagens
func NewMessageStore(config *MessageStoreConfig) (*MessageStore, error) {
	// Criar diretórios de armazenamento se não existirem
	if err := os.MkdirAll(config.StoreDir, 0755); err != nil {
		return nil, err
	}

	store := &MessageStore{
		config:          config,
		pendingMessages: make(map[string]*protocol.Message),
		peerMessages:    make(map[string][]*protocol.Message),
		channelMessages: make(map[string][]*protocol.Message),
		stopChan:        make(chan struct{}),
	}

	// Iniciar limpeza periódica se período de retenção > 0
	if config.RetentionPeriod > 0 {
		store.cleanupTicker = time.NewTicker(1 * time.Hour)
		go store.periodicCleanup()
	}

	return store, nil
}

// StorePendingMessage armazena uma mensagem pendente
func (ms *MessageStore) StorePendingMessage(message *protocol.Message) error {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	// Gerar ID se não existir
	if len(message.SenderID) == 0 || len(message.Content) == 0 {
		return errors.New("mensagem inválida")
	}

	// Usar timestamp como ID se não existir
	messageID := string(message.SenderID) + "_" + time.Now().Format(time.RFC3339Nano)
	ms.pendingMessages[messageID] = message

	return nil
}

// RemovePendingMessage remove uma mensagem pendente
func (ms *MessageStore) RemovePendingMessage(messageID string) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	delete(ms.pendingMessages, messageID)
}

// GetPendingMessages retorna todas as mensagens pendentes
func (ms *MessageStore) GetPendingMessages() []*protocol.Message {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	messages := make([]*protocol.Message, 0, len(ms.pendingMessages))
	for _, msg := range ms.pendingMessages {
		messages = append(messages, msg)
	}

	return messages
}

// StorePeerMessage armazena uma mensagem para um peer específico
func (ms *MessageStore) StorePeerMessage(peerID string, message *protocol.Message) error {
	if peerID == "" {
		return ErrInvalidPeer
	}

	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	// Inicializar slice se necessário
	if _, exists := ms.peerMessages[peerID]; !exists {
		ms.peerMessages[peerID] = make([]*protocol.Message, 0)
	}

	// Adicionar mensagem
	ms.peerMessages[peerID] = append(ms.peerMessages[peerID], message)

	// Limitar número de mensagens por peer
	if len(ms.peerMessages[peerID]) > ms.config.MaxMessagesPerPeer {
		// Remover mensagens mais antigas
		ms.peerMessages[peerID] = ms.peerMessages[peerID][len(ms.peerMessages[peerID])-ms.config.MaxMessagesPerPeer:]
	}

	// Persistir em disco
	return ms.persistPeerMessages(peerID)
}

// GetPeerMessages retorna mensagens para um peer específico
func (ms *MessageStore) GetPeerMessages(peerID string) ([]*protocol.Message, error) {
	if peerID == "" {
		return nil, ErrInvalidPeer
	}

	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	messages, exists := ms.peerMessages[peerID]
	if !exists {
		return make([]*protocol.Message, 0), nil
	}

	return messages, nil
}

// StoreChannelMessage armazena uma mensagem para um canal específico
func (ms *MessageStore) StoreChannelMessage(message *protocol.Message) error {
	if message.Channel == "" {
		return ErrInvalidChannel
	}

	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	// Inicializar slice se necessário
	if _, exists := ms.channelMessages[message.Channel]; !exists {
		ms.channelMessages[message.Channel] = make([]*protocol.Message, 0)
	}

	// Adicionar mensagem
	ms.channelMessages[message.Channel] = append(ms.channelMessages[message.Channel], message)

	// Limitar número de mensagens por canal
	if len(ms.channelMessages[message.Channel]) > ms.config.MaxMessagesPerPeer {
		// Remover mensagens mais antigas
		ms.channelMessages[message.Channel] = ms.channelMessages[message.Channel][len(ms.channelMessages[message.Channel])-ms.config.MaxMessagesPerPeer:]
	}

	// Persistir em disco
	return ms.persistChannelMessages(message.Channel)
}

// GetChannelMessages retorna mensagens para um canal específico
func (ms *MessageStore) GetChannelMessages(channel string) ([]*protocol.Message, error) {
	if channel == "" {
		return nil, ErrInvalidChannel
	}

	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	messages, exists := ms.channelMessages[channel]
	if !exists {
		return make([]*protocol.Message, 0), nil
	}

	return messages, nil
}

// ClearChannelMessages limpa todas as mensagens de um canal
func (ms *MessageStore) ClearChannelMessages(channel string) error {
	if channel == "" {
		return ErrInvalidChannel
	}

	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	delete(ms.channelMessages, channel)
	
	// Remover arquivo persistente
	channelFile := filepath.Join(ms.config.StoreDir, "channel_"+channel+".json")
	if err := os.Remove(channelFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// ClearPeerMessages limpa todas as mensagens de um peer
func (ms *MessageStore) ClearPeerMessages(peerID string) error {
	if peerID == "" {
		return ErrInvalidPeer
	}

	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	delete(ms.peerMessages, peerID)
	
	// Remover arquivo persistente
	peerFile := filepath.Join(ms.config.StoreDir, "peer_"+peerID+".json")
	if err := os.Remove(peerFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// Close fecha o serviço de armazenamento
func (ms *MessageStore) Close() error {
	if ms.cleanupTicker != nil {
		ms.cleanupTicker.Stop()
	}
	
	close(ms.stopChan)
	
	// Persistir todas as mensagens
	ms.mutex.Lock()
	defer ms.mutex.Unlock()
	
	for peerID := range ms.peerMessages {
		if err := ms.persistPeerMessages(peerID); err != nil {
			return err
		}
	}
	
	for channel := range ms.channelMessages {
		if err := ms.persistChannelMessages(channel); err != nil {
			return err
		}
	}
	
	return nil
}

// periodicCleanup executa limpeza periódica de mensagens antigas
func (ms *MessageStore) periodicCleanup() {
	for {
		select {
		case <-ms.cleanupTicker.C:
			ms.cleanupExpiredMessages()
		case <-ms.stopChan:
			return
		}
	}
}

// cleanupExpiredMessages remove mensagens expiradas
func (ms *MessageStore) cleanupExpiredMessages() {
	cutoff := time.Now().Add(-ms.config.RetentionPeriod)
	cutoffNano := cutoff.UnixNano()
	
	ms.mutex.Lock()
	defer ms.mutex.Unlock()
	
	// Limpar mensagens de peers
	for peerID, messages := range ms.peerMessages {
		var newMessages []*protocol.Message
		for _, msg := range messages {
			if msg.Timestamp > uint64(cutoffNano) {
				newMessages = append(newMessages, msg)
			}
		}
		
		if len(newMessages) != len(messages) {
			ms.peerMessages[peerID] = newMessages
			ms.persistPeerMessages(peerID)
		}
	}
	
	// Limpar mensagens de canais
	for channel, messages := range ms.channelMessages {
		var newMessages []*protocol.Message
		for _, msg := range messages {
			if msg.Timestamp > uint64(cutoffNano) {
				newMessages = append(newMessages, msg)
			}
		}
		
		if len(newMessages) != len(messages) {
			ms.channelMessages[channel] = newMessages
			ms.persistChannelMessages(channel)
		}
	}
}

// persistPeerMessages persiste mensagens de peer em disco
func (ms *MessageStore) persistPeerMessages(peerID string) error {
	messages := ms.peerMessages[peerID]
	if len(messages) == 0 {
		return nil
	}
	
	data, err := json.Marshal(messages)
	if err != nil {
		return err
	}
	
	peerFile := filepath.Join(ms.config.StoreDir, "peer_"+peerID+".json")
	return os.WriteFile(peerFile, data, 0644)
}

// persistChannelMessages persiste mensagens de canal em disco
func (ms *MessageStore) persistChannelMessages(channel string) error {
	messages := ms.channelMessages[channel]
	if len(messages) == 0 {
		return nil
	}
	
	data, err := json.Marshal(messages)
	if err != nil {
		return err
	}
	
	channelFile := filepath.Join(ms.config.StoreDir, "channel_"+channel+".json")
	return os.WriteFile(channelFile, data, 0644)
}
