package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/permissionlesstech/bitchat/internal/protocol"
)

func TestMessageStore(t *testing.T) {
	// Criar diretório temporário para testes
	testDir, err := os.MkdirTemp("", "bitchat-test")
	if err != nil {
		t.Fatalf("Erro ao criar diretório temporário: %v", err)
	}
	defer os.RemoveAll(testDir)

	t.Run("Criação do store", func(t *testing.T) {
		config := &MessageStoreConfig{
			DataDir:            testDir,
			MaxMessagesPerPeer: 100,
			MaxMessagesPerChannel: 200,
			RetentionPeriod:    24 * time.Hour,
		}

		store, err := NewMessageStore(config)
		if err != nil {
			t.Fatalf("Erro ao criar MessageStore: %v", err)
		}
		if store == nil {
			t.Fatal("NewMessageStore retornou nil")
		}

		// Verificar se os diretórios foram criados
		dirs := []string{
			filepath.Join(testDir, "messages", "private"),
			filepath.Join(testDir, "messages", "channels"),
			filepath.Join(testDir, "messages", "pending"),
		}

		for _, dir := range dirs {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				t.Errorf("Diretório não foi criado: %s", dir)
			}
		}
	})

	t.Run("Armazenar e recuperar mensagens privadas", func(t *testing.T) {
		config := &MessageStoreConfig{
			DataDir:            filepath.Join(testDir, "private_test"),
			MaxMessagesPerPeer: 100,
			MaxMessagesPerChannel: 200,
			RetentionPeriod:    24 * time.Hour,
		}

		store, err := NewMessageStore(config)
		if err != nil {
			t.Fatalf("Erro ao criar MessageStore: %v", err)
		}

		// Criar mensagens de teste
		messages := []*protocol.Message{
			{
				ID:        "msg1",
				SenderID:  "peer1",
				Content:   []byte("Mensagem de teste 1"),
				Timestamp: uint64(time.Now().Add(-1 * time.Hour).UnixMilli()),
			},
			{
				ID:        "msg2",
				SenderID:  "peer1",
				Content:   []byte("Mensagem de teste 2"),
				Timestamp: uint64(time.Now().UnixMilli()),
			},
			{
				ID:        "msg3",
				SenderID:  "peer2",
				Content:   []byte("Mensagem de outro peer"),
				Timestamp: uint64(time.Now().Add(-30 * time.Minute).UnixMilli()),
			},
		}

		// Armazenar mensagens
		for _, msg := range messages {
			err := store.StorePrivateMessage(msg)
			if err != nil {
				t.Errorf("Erro ao armazenar mensagem privada: %v", err)
			}
		}

		// Recuperar mensagens do peer1
		peer1Messages, err := store.GetPrivateMessages("peer1")
		if err != nil {
			t.Errorf("Erro ao recuperar mensagens privadas: %v", err)
		}
		if len(peer1Messages) != 2 {
			t.Errorf("Número de mensagens do peer1 esperado: 2, obtido: %d", len(peer1Messages))
		}

		// Verificar ordenação por timestamp (mais recente primeiro)
		if peer1Messages[0].ID != "msg2" || peer1Messages[1].ID != "msg1" {
			t.Error("Mensagens não estão ordenadas por timestamp")
		}

		// Recuperar mensagens do peer2
		peer2Messages, err := store.GetPrivateMessages("peer2")
		if err != nil {
			t.Errorf("Erro ao recuperar mensagens privadas: %v", err)
		}
		if len(peer2Messages) != 1 {
			t.Errorf("Número de mensagens do peer2 esperado: 1, obtido: %d", len(peer2Messages))
		}

		// Recuperar mensagens de peer inexistente
		unknownMessages, err := store.GetPrivateMessages("unknown")
		if err != nil {
			t.Errorf("Erro ao recuperar mensagens de peer inexistente: %v", err)
		}
		if len(unknownMessages) != 0 {
			t.Errorf("Número de mensagens de peer desconhecido esperado: 0, obtido: %d", len(unknownMessages))
		}
	})

	t.Run("Armazenar e recuperar mensagens de canal", func(t *testing.T) {
		config := &MessageStoreConfig{
			DataDir:            filepath.Join(testDir, "channel_test"),
			MaxMessagesPerPeer: 100,
			MaxMessagesPerChannel: 200,
			RetentionPeriod:    24 * time.Hour,
		}

		store, err := NewMessageStore(config)
		if err != nil {
			t.Fatalf("Erro ao criar MessageStore: %v", err)
		}

		// Criar mensagens de teste
		channelID := "canal-teste"
		messages := []*protocol.Message{
			{
				ID:        "cmsg1",
				SenderID:  "peer1",
				ChannelID: channelID,
				Content:   []byte("Mensagem de canal 1"),
				Timestamp: uint64(time.Now().Add(-2 * time.Hour).UnixMilli()),
			},
			{
				ID:        "cmsg2",
				SenderID:  "peer2",
				ChannelID: channelID,
				Content:   []byte("Mensagem de canal 2"),
				Timestamp: uint64(time.Now().Add(-1 * time.Hour).UnixMilli()),
			},
			{
				ID:        "cmsg3",
				SenderID:  "peer3",
				ChannelID: channelID,
				Content:   []byte("Mensagem de canal 3"),
				Timestamp: uint64(time.Now().UnixMilli()),
			},
		}

		// Armazenar mensagens
		for _, msg := range messages {
			err := store.StoreChannelMessage(msg)
			if err != nil {
				t.Errorf("Erro ao armazenar mensagem de canal: %v", err)
			}
		}

		// Recuperar mensagens do canal
		channelMessages, err := store.GetChannelMessages(channelID)
		if err != nil {
			t.Errorf("Erro ao recuperar mensagens de canal: %v", err)
		}
		if len(channelMessages) != 3 {
			t.Errorf("Número de mensagens do canal esperado: 3, obtido: %d", len(channelMessages))
		}

		// Verificar ordenação por timestamp (mais recente primeiro)
		if channelMessages[0].ID != "cmsg3" || channelMessages[1].ID != "cmsg2" || channelMessages[2].ID != "cmsg1" {
			t.Error("Mensagens de canal não estão ordenadas por timestamp")
		}

		// Recuperar mensagens de canal inexistente
		unknownChannelMessages, err := store.GetChannelMessages("unknown-channel")
		if err != nil {
			t.Errorf("Erro ao recuperar mensagens de canal inexistente: %v", err)
		}
		if len(unknownChannelMessages) != 0 {
			t.Errorf("Número de mensagens de canal desconhecido esperado: 0, obtido: %d", len(unknownChannelMessages))
		}
	})

	t.Run("Gerenciamento de mensagens pendentes", func(t *testing.T) {
		config := &MessageStoreConfig{
			DataDir:            filepath.Join(testDir, "pending_test"),
			MaxMessagesPerPeer: 100,
			MaxMessagesPerChannel: 200,
			RetentionPeriod:    24 * time.Hour,
		}

		store, err := NewMessageStore(config)
		if err != nil {
			t.Fatalf("Erro ao criar MessageStore: %v", err)
		}

		// Criar pacotes pendentes de teste
		pendingPackets := []*protocol.BitchatPacket{
			{
				ID:          "pending1",
				SenderID:    "self",
				RecipientID: "peer1",
				Type:        protocol.MessageTypePrivate,
				Timestamp:   uint64(time.Now().Add(-30 * time.Minute).UnixMilli()),
				Payload:     []byte("Mensagem pendente 1"),
			},
			{
				ID:          "pending2",
				SenderID:    "self",
				RecipientID: "peer2",
				Type:        protocol.MessageTypePrivate,
				Timestamp:   uint64(time.Now().UnixMilli()),
				Payload:     []byte("Mensagem pendente 2"),
			},
		}

		// Armazenar pacotes pendentes
		for _, packet := range pendingPackets {
			err := store.StorePendingPacket(packet)
			if err != nil {
				t.Errorf("Erro ao armazenar pacote pendente: %v", err)
			}
		}

		// Recuperar todos os pacotes pendentes
		allPending, err := store.GetAllPendingPackets()
		if err != nil {
			t.Errorf("Erro ao recuperar pacotes pendentes: %v", err)
		}
		if len(allPending) != 2 {
			t.Errorf("Número de pacotes pendentes esperado: 2, obtido: %d", len(allPending))
		}

		// Recuperar pacotes pendentes para peer específico
		peer1Pending, err := store.GetPendingPacketsForPeer("peer1")
		if err != nil {
			t.Errorf("Erro ao recuperar pacotes pendentes para peer1: %v", err)
		}
		if len(peer1Pending) != 1 {
			t.Errorf("Número de pacotes pendentes para peer1 esperado: 1, obtido: %d", len(peer1Pending))
		}
		if peer1Pending[0].ID != "pending1" {
			t.Errorf("ID do pacote pendente para peer1 esperado: pending1, obtido: %s", peer1Pending[0].ID)
		}

		// Remover pacote pendente
		err = store.RemovePendingPacket("pending1")
		if err != nil {
			t.Errorf("Erro ao remover pacote pendente: %v", err)
		}

		// Verificar se foi removido
		allPending, _ = store.GetAllPendingPackets()
		if len(allPending) != 1 {
			t.Errorf("Número de pacotes pendentes após remoção esperado: 1, obtido: %d", len(allPending))
		}
		if allPending[0].ID != "pending2" {
			t.Errorf("ID do pacote pendente restante esperado: pending2, obtido: %s", allPending[0].ID)
		}
	})

	t.Run("Limite de mensagens por peer", func(t *testing.T) {
		// Configurar limite baixo para teste
		config := &MessageStoreConfig{
			DataDir:            filepath.Join(testDir, "limit_test"),
			MaxMessagesPerPeer: 3, // Limite baixo para teste
			MaxMessagesPerChannel: 200,
			RetentionPeriod:    24 * time.Hour,
		}

		store, err := NewMessageStore(config)
		if err != nil {
			t.Fatalf("Erro ao criar MessageStore: %v", err)
		}

		// Criar mais mensagens que o limite
		for i := 0; i < 5; i++ {
			msg := &protocol.Message{
				ID:        "limit-msg-" + string(rune('1'+i)),
				SenderID:  "limit-peer",
				Content:   []byte("Mensagem de teste limite " + string(rune('1'+i))),
				Timestamp: uint64(time.Now().Add(time.Duration(i) * time.Minute).UnixMilli()),
			}
			err := store.StorePrivateMessage(msg)
			if err != nil {
				t.Errorf("Erro ao armazenar mensagem para teste de limite: %v", err)
			}
		}

		// Recuperar mensagens
		messages, err := store.GetPrivateMessages("limit-peer")
		if err != nil {
			t.Errorf("Erro ao recuperar mensagens para teste de limite: %v", err)
		}

		// Verificar se apenas o limite foi mantido
		if len(messages) != 3 {
			t.Errorf("Número de mensagens após limite esperado: 3, obtido: %d", len(messages))
		}

		// Verificar se as mensagens mais recentes foram mantidas
		expectedIDs := []string{"limit-msg-5", "limit-msg-4", "limit-msg-3"}
		for i, msg := range messages {
			if msg.ID != expectedIDs[i] {
				t.Errorf("ID da mensagem %d esperado: %s, obtido: %s", i, expectedIDs[i], msg.ID)
			}
		}
	})

	t.Run("Limpeza por período de retenção", func(t *testing.T) {
		// Configurar período de retenção curto para teste
		config := &MessageStoreConfig{
			DataDir:            filepath.Join(testDir, "retention_test"),
			MaxMessagesPerPeer: 100,
			MaxMessagesPerChannel: 200,
			RetentionPeriod:    1 * time.Hour,
		}

		store, err := NewMessageStore(config)
		if err != nil {
			t.Fatalf("Erro ao criar MessageStore: %v", err)
		}

		// Criar mensagens com timestamps variados
		messages := []*protocol.Message{
			{
				ID:        "recent",
				SenderID:  "retention-peer",
				Content:   []byte("Mensagem recente"),
				Timestamp: uint64(time.Now().UnixMilli()),
			},
			{
				ID:        "old",
				SenderID:  "retention-peer",
				Content:   []byte("Mensagem antiga"),
				Timestamp: uint64(time.Now().Add(-2 * time.Hour).UnixMilli()), // Mais antiga que o período de retenção
			},
		}

		// Armazenar mensagens
		for _, msg := range messages {
			err := store.StorePrivateMessage(msg)
			if err != nil {
				t.Errorf("Erro ao armazenar mensagem para teste de retenção: %v", err)
			}
		}

		// Forçar limpeza
		err = store.CleanupExpiredMessages()
		if err != nil {
			t.Errorf("Erro ao limpar mensagens expiradas: %v", err)
		}

		// Recuperar mensagens
		remainingMessages, err := store.GetPrivateMessages("retention-peer")
		if err != nil {
			t.Errorf("Erro ao recuperar mensagens após limpeza: %v", err)
		}

		// Verificar se apenas a mensagem recente foi mantida
		if len(remainingMessages) != 1 {
			t.Errorf("Número de mensagens após limpeza esperado: 1, obtido: %d", len(remainingMessages))
		}
		if len(remainingMessages) > 0 && remainingMessages[0].ID != "recent" {
			t.Errorf("ID da mensagem restante esperado: recent, obtido: %s", remainingMessages[0].ID)
		}
	})
}
