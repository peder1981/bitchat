package mesh

import (
	"testing"
	"time"

	"github.com/permissionlesstech/bitchat/internal/protocol"
)

func TestMessageRouter(t *testing.T) {
	t.Run("Criação do roteador", func(t *testing.T) {
		router := NewMessageRouter()
		if router == nil {
			t.Fatal("NewMessageRouter retornou nil")
		}
		
		if router.GetDefaultTTL() != 5 {
			t.Errorf("TTL padrão esperado: 5, obtido: %d", router.GetDefaultTTL())
		}
	})
	
	t.Run("Configuração de TTL", func(t *testing.T) {
		router := NewMessageRouter()
		
		// Alterar TTL
		router.SetDefaultTTL(10)
		
		if router.GetDefaultTTL() != 10 {
			t.Errorf("TTL padrão esperado após alteração: 10, obtido: %d", router.GetDefaultTTL())
		}
	})
	
	t.Run("Deduplicação de mensagens", func(t *testing.T) {
		router := NewMessageRouter()
		
		// Criar pacote de teste
		packet := &protocol.BitchatPacket{
			ID:        "test-message-1",
			TTL:       5,
			Timestamp: uint64(time.Now().UnixMilli()),
		}
		
		// Primeira vez deve processar
		if !router.ShouldProcess(packet) {
			t.Error("Primeira mensagem deveria ser processada")
		}
		
		// Segunda vez deve descartar (duplicada)
		if router.ShouldProcess(packet) {
			t.Error("Mensagem duplicada não deveria ser processada")
		}
		
		// Pacote com TTL zero não deve ser processado
		zeroTTLPacket := &protocol.BitchatPacket{
			ID:        "test-message-2",
			TTL:       0,
			Timestamp: uint64(time.Now().UnixMilli()),
		}
		
		if router.ShouldProcess(zeroTTLPacket) {
			t.Error("Pacote com TTL zero não deveria ser processado")
		}
	})
	
	t.Run("Diminuição de TTL", func(t *testing.T) {
		router := NewMessageRouter()
		
		// Pacote com TTL > 1
		validPacket := &protocol.BitchatPacket{
			ID:        "test-ttl-1",
			TTL:       3,
			Timestamp: uint64(time.Now().UnixMilli()),
		}
		
		// Deve diminuir TTL e retornar true
		if !router.DecreaseAndCheckTTL(validPacket) {
			t.Error("DecreaseAndCheckTTL deveria retornar true para TTL > 1")
		}
		
		if validPacket.TTL != 2 {
			t.Errorf("TTL esperado após diminuição: 2, obtido: %d", validPacket.TTL)
		}
		
		// Pacote com TTL = 1
		expiringPacket := &protocol.BitchatPacket{
			ID:        "test-ttl-2",
			TTL:       1,
			Timestamp: uint64(time.Now().UnixMilli()),
		}
		
		// Deve retornar false (expirado)
		if router.DecreaseAndCheckTTL(expiringPacket) {
			t.Error("DecreaseAndCheckTTL deveria retornar false para TTL = 1")
		}
	})
	
	t.Run("Tabela de roteamento", func(t *testing.T) {
		router := NewMessageRouter()
		
		// Adicionar rotas
		router.UpdateRoutingInfo("peer1", "", 80) // Conexão direta
		router.UpdateRoutingInfo("peer2", "peer1", 70) // Via peer1
		router.UpdateRoutingInfo("peer3", "peer1", 60) // Via peer1
		
		// Verificar rotas
		nextHop, exists := router.GetNextHop("peer1")
		if !exists {
			t.Error("Rota para peer1 deveria existir")
		}
		if nextHop != "peer1" {
			t.Errorf("NextHop para peer1 esperado: peer1, obtido: %s", nextHop)
		}
		
		nextHop, exists = router.GetNextHop("peer2")
		if !exists {
			t.Error("Rota para peer2 deveria existir")
		}
		if nextHop != "peer1" {
			t.Errorf("NextHop para peer2 esperado: peer1, obtido: %s", nextHop)
		}
		
		// Verificar rota inexistente
		_, exists = router.GetNextHop("unknown")
		if exists {
			t.Error("Não deveria existir rota para peer desconhecido")
		}
		
		// Atualizar rota com métrica melhor
		router.UpdateRoutingInfo("peer2", "peer3", 90)
		
		nextHop, _ = router.GetNextHop("peer2")
		if nextHop != "peer3" {
			t.Errorf("NextHop para peer2 após atualização esperado: peer3, obtido: %s", nextHop)
		}
		
		// Atualizar rota com métrica pior (não deve alterar)
		router.UpdateRoutingInfo("peer2", "peer1", 50)
		
		nextHop, _ = router.GetNextHop("peer2")
		if nextHop != "peer3" {
			t.Errorf("NextHop para peer2 não deveria mudar para rota pior, obtido: %s", nextHop)
		}
	})
	
	t.Run("Remoção de peer", func(t *testing.T) {
		router := NewMessageRouter()
		
		// Configurar rotas
		router.UpdateRoutingInfo("peer1", "", 80)
		router.UpdateRoutingInfo("peer2", "peer1", 70)
		router.UpdateRoutingInfo("peer3", "", 90)
		router.UpdateRoutingInfo("peer4", "peer3", 85)
		
		// Remover peer1
		router.RemovePeer("peer1")
		
		// Verificar se peer1 foi removido
		_, exists := router.GetNextHop("peer1")
		if exists {
			t.Error("peer1 deveria ter sido removido")
		}
		
		// Verificar se rotas via peer1 foram removidas
		_, exists = router.GetNextHop("peer2")
		if exists {
			t.Error("peer2 (roteado via peer1) deveria ter sido removido")
		}
		
		// Verificar se outras rotas permanecem
		_, exists = router.GetNextHop("peer3")
		if !exists {
			t.Error("peer3 não deveria ter sido removido")
		}
		
		_, exists = router.GetNextHop("peer4")
		if !exists {
			t.Error("peer4 não deveria ter sido removido")
		}
	})
	
	t.Run("Listar peers", func(t *testing.T) {
		router := NewMessageRouter()
		
		// Configurar rotas
		router.UpdateRoutingInfo("peer1", "", 80)
		router.UpdateRoutingInfo("peer2", "peer1", 70)
		router.UpdateRoutingInfo("peer3", "", 90)
		
		// Obter todos os peers
		allPeers := router.GetAllPeers()
		if len(allPeers) != 3 {
			t.Errorf("Número de peers esperado: 3, obtido: %d", len(allPeers))
		}
		
		// Verificar se todos os peers estão na lista
		peerMap := make(map[string]bool)
		for _, peer := range allPeers {
			peerMap[peer] = true
		}
		
		if !peerMap["peer1"] || !peerMap["peer2"] || !peerMap["peer3"] {
			t.Error("GetAllPeers não retornou todos os peers esperados")
		}
		
		// Obter peers diretos
		directPeers := router.GetDirectPeers()
		if len(directPeers) != 2 {
			t.Errorf("Número de peers diretos esperado: 2, obtido: %d", len(directPeers))
		}
		
		// Verificar se apenas peers diretos estão na lista
		directMap := make(map[string]bool)
		for _, peer := range directPeers {
			directMap[peer] = true
		}
		
		if !directMap["peer1"] || !directMap["peer3"] || directMap["peer2"] {
			t.Error("GetDirectPeers não retornou os peers diretos corretos")
		}
	})
	
	t.Run("Preparar pacote para envio", func(t *testing.T) {
		router := NewMessageRouter()
		router.SetDefaultTTL(8)
		
		// Pacote sem TTL
		packet := &protocol.BitchatPacket{
			ID:        "outgoing-test",
			Timestamp: uint64(time.Now().UnixMilli()),
		}
		
		router.PrepareOutgoingPacket(packet)
		
		if packet.TTL != 8 {
			t.Errorf("TTL esperado após PrepareOutgoingPacket: 8, obtido: %d", packet.TTL)
		}
		
		// Pacote com TTL já definido
		packet2 := &protocol.BitchatPacket{
			ID:        "outgoing-test-2",
			TTL:       3,
			Timestamp: uint64(time.Now().UnixMilli()),
		}
		
		router.PrepareOutgoingPacket(packet2)
		
		if packet2.TTL != 3 {
			t.Errorf("TTL não deveria ser alterado se já definido, esperado: 3, obtido: %d", packet2.TTL)
		}
	})
	
	t.Run("Limpar roteador", func(t *testing.T) {
		router := NewMessageRouter()
		
		// Configurar rotas
		router.UpdateRoutingInfo("peer1", "", 80)
		router.UpdateRoutingInfo("peer2", "peer1", 70)
		
		// Marcar mensagem como processada
		packet := &protocol.BitchatPacket{
			ID:        "clear-test",
			TTL:       5,
			Timestamp: uint64(time.Now().UnixMilli()),
		}
		
		router.MarkProcessed(packet)
		
		// Limpar roteador
		router.Clear()
		
		// Verificar se rotas foram removidas
		_, exists := router.GetNextHop("peer1")
		if exists {
			t.Error("Rotas deveriam ter sido removidas após Clear")
		}
		
		// Verificar se cache de mensagens foi limpo
		if !router.ShouldProcess(packet) {
			t.Error("Cache de mensagens processadas deveria ter sido limpo após Clear")
		}
	})
}
