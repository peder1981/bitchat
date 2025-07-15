package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/permissionlesstech/bitchat/internal/protocol"
	"github.com/permissionlesstech/bitchat/pkg/utils"
)

// MessageStore gerencia o armazenamento persistente de mensagens
type MessageStore struct {
	dataDir         string
	channelMessages map[string][]*protocol.BitchatMessage // canal -> mensagens
	privateMessages map[string][]*protocol.BitchatMessage // peerID -> mensagens
	pendingMessages map[string]*protocol.BitchatPacket    // messageID -> pacote
	mutex           sync.RWMutex
	maxMessages     int
	retentionPeriod time.Duration
}

// NewMessageStore cria um novo armazenamento de mensagens
func NewMessageStore(dataDir string) (*MessageStore, error) {
	// Garantir que o diretório de dados existe
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("erro ao criar diretório de dados: %v", err)
	}

	store := &MessageStore{
		dataDir:         dataDir,
		channelMessages: make(map[string][]*protocol.BitchatMessage),
		privateMessages: make(map[string][]*protocol.BitchatMessage),
		pendingMessages: make(map[string]*protocol.BitchatPacket),
		maxMessages:     1000,                    // Máximo de mensagens por canal/peer
		retentionPeriod: 30 * 24 * time.Hour,     // 30 dias de retenção padrão
	}

	// Carregar mensagens salvas
	if err := store.loadMessages(); err != nil {
		fmt.Printf("Aviso: erro ao carregar mensagens: %v\n", err)
	}

	return store, nil
}

// AddChannelMessage adiciona uma mensagem ao histórico de um canal
func (ms *MessageStore) AddChannelMessage(channel string, message *protocol.BitchatMessage) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	// Criar slice se não existir
	if _, ok := ms.channelMessages[channel]; !ok {
		ms.channelMessages[channel] = make([]*protocol.BitchatMessage, 0)
	}

	// Adicionar mensagem
	ms.channelMessages[channel] = append(ms.channelMessages[channel], message)

	// Limitar número de mensagens
	if len(ms.channelMessages[channel]) > ms.maxMessages {
		// Remover mensagem mais antiga
		ms.channelMessages[channel] = ms.channelMessages[channel][1:]
	}

	// Salvar em background
	go ms.saveChannelMessages(channel)
}

// AddPrivateMessage adiciona uma mensagem ao histórico de mensagens privadas
func (ms *MessageStore) AddPrivateMessage(peerID string, message *protocol.BitchatMessage) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	// Criar slice se não existir
	if _, ok := ms.privateMessages[peerID]; !ok {
		ms.privateMessages[peerID] = make([]*protocol.BitchatMessage, 0)
	}

	// Adicionar mensagem
	ms.privateMessages[peerID] = append(ms.privateMessages[peerID], message)

	// Limitar número de mensagens
	if len(ms.privateMessages[peerID]) > ms.maxMessages {
		// Remover mensagem mais antiga
		ms.privateMessages[peerID] = ms.privateMessages[peerID][1:]
	}

	// Salvar em background
	go ms.savePrivateMessages(peerID)
}

// GetChannelMessages retorna as mensagens de um canal
func (ms *MessageStore) GetChannelMessages(channel string) []*protocol.BitchatMessage {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	if messages, ok := ms.channelMessages[channel]; ok {
		return messages
	}

	return []*protocol.BitchatMessage{}
}

// GetPrivateMessages retorna as mensagens privadas com um peer
func (ms *MessageStore) GetPrivateMessages(peerID string) []*protocol.BitchatMessage {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	if messages, ok := ms.privateMessages[peerID]; ok {
		return messages
	}

	return []*protocol.BitchatMessage{}
}

// ClearChannelMessages limpa o histórico de mensagens de um canal
func (ms *MessageStore) ClearChannelMessages(channel string) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	delete(ms.channelMessages, channel)

	// Remover arquivo de mensagens
	filename := filepath.Join(ms.dataDir, fmt.Sprintf("channel_%s.json", utils.Hash(channel)))
	os.Remove(filename)
}

// ClearPrivateMessages limpa o histórico de mensagens privadas com um peer
func (ms *MessageStore) ClearPrivateMessages(peerID string) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	delete(ms.privateMessages, peerID)

	// Remover arquivo de mensagens
	filename := filepath.Join(ms.dataDir, fmt.Sprintf("private_%s.json", peerID))
	os.Remove(filename)
}

// AddPendingMessage adiciona uma mensagem pendente para entrega posterior
func (ms *MessageStore) AddPendingMessage(messageID string, packet *protocol.BitchatPacket) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	ms.pendingMessages[messageID] = packet

	// Salvar em background
	go ms.savePendingMessages()
}

// GetPendingMessages retorna todas as mensagens pendentes
func (ms *MessageStore) GetPendingMessages() map[string]*protocol.BitchatPacket {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()

	// Criar cópia para evitar problemas de concorrência
	result := make(map[string]*protocol.BitchatPacket, len(ms.pendingMessages))
	for id, packet := range ms.pendingMessages {
		result[id] = packet
	}

	return result
}

// RemovePendingMessage remove uma mensagem pendente
func (ms *MessageStore) RemovePendingMessage(messageID string) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	delete(ms.pendingMessages, messageID)

	// Salvar em background
	go ms.savePendingMessages()
}

// SetMaxMessages define o número máximo de mensagens por canal/peer
func (ms *MessageStore) SetMaxMessages(max int) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	ms.maxMessages = max
}

// SetRetentionPeriod define o período de retenção de mensagens
func (ms *MessageStore) SetRetentionPeriod(period time.Duration) {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	ms.retentionPeriod = period
}

// CleanupOldMessages remove mensagens mais antigas que o período de retenção
func (ms *MessageStore) CleanupOldMessages() {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	cutoff := time.Now().Add(-ms.retentionPeriod)

	// Limpar mensagens de canais
	for channel, messages := range ms.channelMessages {
		var newMessages []*protocol.BitchatMessage
		for _, msg := range messages {
			timestamp := time.UnixMilli(int64(msg.Timestamp))
			if timestamp.After(cutoff) {
				newMessages = append(newMessages, msg)
			}
		}
		ms.channelMessages[channel] = newMessages
	}

	// Limpar mensagens privadas
	for peerID, messages := range ms.privateMessages {
		var newMessages []*protocol.BitchatMessage
		for _, msg := range messages {
			timestamp := time.UnixMilli(int64(msg.Timestamp))
			if timestamp.After(cutoff) {
				newMessages = append(newMessages, msg)
			}
		}
		ms.privateMessages[peerID] = newMessages
	}

	// Salvar alterações
	go ms.saveAllMessages()
}

// Métodos internos para persistência

func (ms *MessageStore) loadMessages() error {
	// Carregar mensagens de canais
	channelFiles, err := filepath.Glob(filepath.Join(ms.dataDir, "channel_*.json"))
	if err != nil {
		return err
	}

	for _, file := range channelFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Erro ao ler arquivo %s: %v\n", file, err)
			continue
		}

		var messages []*protocol.BitchatMessage
		if err := json.Unmarshal(data, &messages); err != nil {
			fmt.Printf("Erro ao decodificar mensagens do arquivo %s: %v\n", file, err)
			continue
		}

		// Extrair nome do canal do nome do arquivo
		base := filepath.Base(file)
		channel := base[8 : len(base)-5] // Remover "channel_" e ".json"
		ms.channelMessages[channel] = messages
	}

	// Carregar mensagens privadas
	privateFiles, err := filepath.Glob(filepath.Join(ms.dataDir, "private_*.json"))
	if err != nil {
		return err
	}

	for _, file := range privateFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Erro ao ler arquivo %s: %v\n", file, err)
			continue
		}

		var messages []*protocol.BitchatMessage
		if err := json.Unmarshal(data, &messages); err != nil {
			fmt.Printf("Erro ao decodificar mensagens do arquivo %s: %v\n", file, err)
			continue
		}

		// Extrair ID do peer do nome do arquivo
		base := filepath.Base(file)
		peerID := base[8 : len(base)-5] // Remover "private_" e ".json"
		ms.privateMessages[peerID] = messages
	}

	// Carregar mensagens pendentes
	pendingFile := filepath.Join(ms.dataDir, "pending.json")
	if _, err := os.Stat(pendingFile); err == nil {
		data, err := os.ReadFile(pendingFile)
		if err != nil {
			fmt.Printf("Erro ao ler arquivo de mensagens pendentes: %v\n", err)
		} else {
			var pendingData map[string][]byte
			if err := json.Unmarshal(data, &pendingData); err != nil {
				fmt.Printf("Erro ao decodificar mensagens pendentes: %v\n", err)
			} else {
				for id, packetData := range pendingData {
					packet, err := protocol.Decode(packetData)
					if err != nil {
						fmt.Printf("Erro ao decodificar pacote pendente %s: %v\n", id, err)
						continue
					}
					ms.pendingMessages[id] = packet
				}
			}
		}
	}

	return nil
}

func (ms *MessageStore) saveChannelMessages(channel string) {
	ms.mutex.RLock()
	messages, ok := ms.channelMessages[channel]
	ms.mutex.RUnlock()

	if !ok {
		return
	}

	// Serializar mensagens
	data, err := json.Marshal(messages)
	if err != nil {
		fmt.Printf("Erro ao serializar mensagens do canal %s: %v\n", channel, err)
		return
	}

	// Salvar em arquivo
	filename := filepath.Join(ms.dataDir, fmt.Sprintf("channel_%s.json", utils.Hash(channel)))
	if err := os.WriteFile(filename, data, 0600); err != nil {
		fmt.Printf("Erro ao salvar mensagens do canal %s: %v\n", channel, err)
	}
}

func (ms *MessageStore) savePrivateMessages(peerID string) {
	ms.mutex.RLock()
	messages, ok := ms.privateMessages[peerID]
	ms.mutex.RUnlock()

	if !ok {
		return
	}

	// Serializar mensagens
	data, err := json.Marshal(messages)
	if err != nil {
		fmt.Printf("Erro ao serializar mensagens privadas com %s: %v\n", peerID, err)
		return
	}

	// Salvar em arquivo
	filename := filepath.Join(ms.dataDir, fmt.Sprintf("private_%s.json", peerID))
	if err := os.WriteFile(filename, data, 0600); err != nil {
		fmt.Printf("Erro ao salvar mensagens privadas com %s: %v\n", peerID, err)
	}
}

func (ms *MessageStore) savePendingMessages() {
	ms.mutex.RLock()
	pendingMessages := ms.pendingMessages
	ms.mutex.RUnlock()

	// Serializar pacotes pendentes
	pendingData := make(map[string][]byte)
	for id, packet := range pendingMessages {
		data, err := protocol.Encode(packet)
		if err != nil {
			fmt.Printf("Erro ao codificar pacote pendente %s: %v\n", id, err)
			continue
		}
		pendingData[id] = data
	}

	// Serializar mapa
	data, err := json.Marshal(pendingData)
	if err != nil {
		fmt.Printf("Erro ao serializar mensagens pendentes: %v\n", err)
		return
	}

	// Salvar em arquivo
	filename := filepath.Join(ms.dataDir, "pending.json")
	if err := os.WriteFile(filename, data, 0600); err != nil {
		fmt.Printf("Erro ao salvar mensagens pendentes: %v\n", err)
	}
}

func (ms *MessageStore) saveAllMessages() {
	// Salvar mensagens de canais
	ms.mutex.RLock()
	channels := make([]string, 0, len(ms.channelMessages))
	for channel := range ms.channelMessages {
		channels = append(channels, channel)
	}
	ms.mutex.RUnlock()

	for _, channel := range channels {
		ms.saveChannelMessages(channel)
	}

	// Salvar mensagens privadas
	ms.mutex.RLock()
	peers := make([]string, 0, len(ms.privateMessages))
	for peerID := range ms.privateMessages {
		peers = append(peers, peerID)
	}
	ms.mutex.RUnlock()

	for _, peerID := range peers {
		ms.savePrivateMessages(peerID)
	}

	// Salvar mensagens pendentes
	ms.savePendingMessages()
}
