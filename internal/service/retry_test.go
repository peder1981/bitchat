package service

import (
	"sync"
	"testing"
	"time"

	"github.com/permissionlesstech/bitchat/internal/protocol"
)

func TestRetryService(t *testing.T) {
	t.Run("Criação do serviço", func(t *testing.T) {
		// Função mock para envio de pacotes
		sendFunc := func(packet *protocol.BitchatPacket, targetPeerID string) error {
			return nil
		}

		// Criar serviço com configuração padrão
		rs := NewRetryService(nil, sendFunc)
		if rs == nil {
			t.Fatal("NewRetryService retornou nil")
		}

		// Criar serviço com configuração personalizada
		config := &RetryConfig{
			MaxRetries:     3,
			InitialBackoff: 1 * time.Second,
			BackoffFactor:  2.0,
			MaxBackoff:     10 * time.Second,
			MaxRetryTime:   1 * time.Minute,
		}

		rs = NewRetryService(config, sendFunc)
		if rs == nil {
			t.Fatal("NewRetryService com config personalizada retornou nil")
		}
	})

	t.Run("Adicionar e marcar entregue", func(t *testing.T) {
		// Variáveis para rastrear chamadas
		var (
			sendCount     int
			callbackCalled bool
			callbackSuccess bool
			mutex          sync.Mutex
		)

		// Função mock para envio de pacotes
		sendFunc := func(packet *protocol.BitchatPacket, targetPeerID string) error {
			mutex.Lock()
			sendCount++
			mutex.Unlock()
			return nil
		}

		// Configuração com tempos curtos para teste
		config := &RetryConfig{
			MaxRetries:     3,
			InitialBackoff: 50 * time.Millisecond,
			BackoffFactor:  1.5,
			MaxBackoff:     200 * time.Millisecond,
			MaxRetryTime:   1 * time.Second,
		}

		// Criar serviço
		rs := NewRetryService(config, sendFunc)
		rs.Start()
		defer rs.Stop()

		// Criar pacote de teste
		packet := &protocol.BitchatPacket{
			ID:        "test-retry-1",
			Timestamp: uint64(time.Now().UnixMilli()),
		}

		// Callback para verificar entrega
		callback := func(messageID string, success bool, info *protocol.DeliveryInfo) {
			mutex.Lock()
			callbackCalled = true
			callbackSuccess = success
			mutex.Unlock()
		}

		// Adicionar pacote para retry
		rs.AddRetry(packet, "peer1", callback)

		// Verificar contagem inicial de pendentes
		if count := rs.GetPendingCount(); count != 1 {
			t.Errorf("Contagem inicial de pendentes esperada: 1, obtida: %d", count)
		}

		// Verificar se o pacote está na lista de pendentes
		pendingMessages := rs.GetPendingMessages()
		if len(pendingMessages) != 1 {
			t.Errorf("Número de mensagens pendentes esperado: 1, obtido: %d", len(pendingMessages))
		}

		// Marcar como entregue
		rs.MarkDelivered("test-retry-1")

		// Verificar se foi removido da lista de pendentes
		if count := rs.GetPendingCount(); count != 0 {
			t.Errorf("Contagem de pendentes após entrega esperada: 0, obtida: %d", count)
		}

		// Verificar se o callback foi chamado com sucesso
		time.Sleep(10 * time.Millisecond) // Pequena espera para garantir que o callback foi processado
		mutex.Lock()
		if !callbackCalled {
			t.Error("Callback não foi chamado após marcar como entregue")
		}
		if !callbackSuccess {
			t.Error("Callback deveria indicar sucesso")
		}
		mutex.Unlock()
	})

	t.Run("Retry automático", func(t *testing.T) {
		// Variáveis para rastrear chamadas
		var (
			sendCount int
			mutex     sync.Mutex
		)

		// Função mock para envio de pacotes
		sendFunc := func(packet *protocol.BitchatPacket, targetPeerID string) error {
			mutex.Lock()
			sendCount++
			mutex.Unlock()
			return nil
		}

		// Configuração com tempos curtos para teste
		config := &RetryConfig{
			MaxRetries:     2,
			InitialBackoff: 50 * time.Millisecond,
			BackoffFactor:  1.0, // Sem crescimento para simplificar o teste
			MaxBackoff:     50 * time.Millisecond,
			MaxRetryTime:   500 * time.Millisecond,
		}

		// Criar serviço
		rs := NewRetryService(config, sendFunc)
		rs.Start()
		defer rs.Stop()

		// Criar pacote de teste
		packet := &protocol.BitchatPacket{
			ID:        "test-retry-auto",
			Timestamp: uint64(time.Now().UnixMilli()),
		}

		// Adicionar pacote para retry
		rs.AddRetry(packet, "peer1", nil)

		// Esperar tempo suficiente para pelo menos 2 retries
		time.Sleep(150 * time.Millisecond)

		// Verificar se houve pelo menos 3 envios (inicial + 2 retries)
		mutex.Lock()
		if sendCount < 3 {
			t.Errorf("Número mínimo de envios esperado: 3, obtido: %d", sendCount)
		}
		mutex.Unlock()
	})

	t.Run("Falha após máximo de tentativas", func(t *testing.T) {
		// Variáveis para rastrear chamadas
		var (
			callbackCalled bool
			callbackSuccess bool
			callbackAttempts int
			mutex          sync.Mutex
		)

		// Função mock para envio de pacotes
		sendFunc := func(packet *protocol.BitchatPacket, targetPeerID string) error {
			return nil
		}

		// Configuração com tempos curtos para teste
		config := &RetryConfig{
			MaxRetries:     2,
			InitialBackoff: 20 * time.Millisecond,
			BackoffFactor:  1.0,
			MaxBackoff:     20 * time.Millisecond,
			MaxRetryTime:   100 * time.Millisecond,
		}

		// Criar serviço
		rs := NewRetryService(config, sendFunc)
		rs.Start()
		defer rs.Stop()

		// Criar pacote de teste
		packet := &protocol.BitchatPacket{
			ID:        "test-retry-fail",
			Timestamp: uint64(time.Now().UnixMilli()),
		}

		// Callback para verificar falha
		callback := func(messageID string, success bool, info *protocol.DeliveryInfo) {
			mutex.Lock()
			callbackCalled = true
			callbackSuccess = success
			if info != nil {
				callbackAttempts = info.Attempts
			}
			mutex.Unlock()
		}

		// Adicionar pacote para retry
		rs.AddRetry(packet, "peer1", callback)

		// Esperar tempo suficiente para exceder o máximo de tentativas
		time.Sleep(150 * time.Millisecond)

		// Verificar se o callback foi chamado com falha
		mutex.Lock()
		if !callbackCalled {
			t.Error("Callback não foi chamado após falha")
		}
		if callbackSuccess {
			t.Error("Callback deveria indicar falha")
		}
		if callbackAttempts != 3 { // 1 inicial + 2 retries
			t.Errorf("Número de tentativas esperado: 3, obtido: %d", callbackAttempts)
		}
		mutex.Unlock()
	})

	t.Run("Limpar retries", func(t *testing.T) {
		// Função mock para envio de pacotes
		sendFunc := func(packet *protocol.BitchatPacket, targetPeerID string) error {
			return nil
		}

		// Criar serviço
		rs := NewRetryService(nil, sendFunc)
		rs.Start()
		defer rs.Stop()

		// Adicionar vários pacotes
		for i := 0; i < 3; i++ {
			packet := &protocol.BitchatPacket{
				ID:        "test-clear-" + string(rune('A'+i)),
				Timestamp: uint64(time.Now().UnixMilli()),
			}
			rs.AddRetry(packet, "peer1", nil)
		}

		// Verificar contagem inicial
		if count := rs.GetPendingCount(); count != 3 {
			t.Errorf("Contagem inicial esperada: 3, obtida: %d", count)
		}

		// Limpar retries
		rs.ClearRetries()

		// Verificar se todos foram removidos
		if count := rs.GetPendingCount(); count != 0 {
			t.Errorf("Contagem após limpar esperada: 0, obtida: %d", count)
		}
	})

	t.Run("Adicionar duplicado", func(t *testing.T) {
		// Função mock para envio de pacotes
		sendFunc := func(packet *protocol.BitchatPacket, targetPeerID string) error {
			return nil
		}

		// Criar serviço
		rs := NewRetryService(nil, sendFunc)

		// Criar pacote de teste
		packet := &protocol.BitchatPacket{
			ID:        "test-duplicate",
			Timestamp: uint64(time.Now().UnixMilli()),
		}

		// Adicionar pacote duas vezes
		rs.AddRetry(packet, "peer1", nil)
		rs.AddRetry(packet, "peer2", nil) // Mesmo ID, peer diferente

		// Verificar se apenas um foi adicionado
		if count := rs.GetPendingCount(); count != 1 {
			t.Errorf("Contagem esperada após tentativa duplicada: 1, obtida: %d", count)
		}
	})
}
