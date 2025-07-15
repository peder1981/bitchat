package utils

import (
	"testing"
	"time"
)

func TestExpiringSet(t *testing.T) {
	// Criar um conjunto com expiração curta para testes
	ttl := 100 * time.Millisecond
	cleanupInterval := 50 * time.Millisecond
	es := NewExpiringSet(ttl, cleanupInterval)
	defer es.Stop()

	t.Run("Adicionar e verificar itens", func(t *testing.T) {
		// Adicionar itens
		if !es.Add("item1") {
			t.Error("Falha ao adicionar item1")
		}
		if !es.Add("item2") {
			t.Error("Falha ao adicionar item2")
		}

		// Verificar se os itens existem
		if !es.Contains("item1") {
			t.Error("item1 deveria existir")
		}
		if !es.Contains("item2") {
			t.Error("item2 deveria existir")
		}
		if es.Contains("item3") {
			t.Error("item3 não deveria existir")
		}

		// Verificar tamanho
		if es.Size() != 2 {
			t.Errorf("Tamanho esperado: 2, obtido: %d", es.Size())
		}

		// Tentar adicionar item duplicado
		if es.Add("item1") {
			t.Error("Não deveria permitir adicionar item1 novamente")
		}
	})

	t.Run("Remover itens", func(t *testing.T) {
		es.Add("item3")
		es.Add("item4")

		// Remover item
		es.Remove("item3")
		if es.Contains("item3") {
			t.Error("item3 não deveria existir após remoção")
		}
		if !es.Contains("item4") {
			t.Error("item4 deveria existir")
		}
	})

	t.Run("Expiração de itens", func(t *testing.T) {
		es.Add("temp")

		// Esperar pela expiração
		time.Sleep(ttl + 10*time.Millisecond)

		// Verificar se o item expirou
		if es.Contains("temp") {
			t.Error("temp deveria ter expirado")
		}
	})

	t.Run("Atualizar expiração", func(t *testing.T) {
		es.Add("update")

		// Esperar um pouco, mas não o suficiente para expirar
		time.Sleep(ttl / 2)

		// Atualizar expiração
		if !es.UpdateExpiry("update") {
			t.Error("Falha ao atualizar expiração")
		}

		// Esperar pelo tempo original de expiração
		time.Sleep(ttl * 3 / 4)

		// O item não deveria ter expirado ainda
		if !es.Contains("update") {
			t.Error("update não deveria ter expirado após atualização")
		}

		// Esperar mais para garantir que expire
		time.Sleep(ttl)
		if es.Contains("update") {
			t.Error("update deveria ter expirado eventualmente")
		}
	})

	t.Run("Limpar conjunto", func(t *testing.T) {
		es.Add("clear1")
		es.Add("clear2")

		// Limpar conjunto
		es.Clear()

		// Verificar se está vazio
		if es.Size() != 0 {
			t.Errorf("Tamanho esperado após Clear: 0, obtido: %d", es.Size())
		}
		if es.Contains("clear1") || es.Contains("clear2") {
			t.Error("Itens não deveriam existir após Clear")
		}
	})

	t.Run("GetAll", func(t *testing.T) {
		es.Clear()
		es.Add("all1")
		es.Add("all2")
		es.Add("all3")

		// Obter todos os itens
		items := es.GetAll()
		if len(items) != 3 {
			t.Errorf("GetAll deveria retornar 3 itens, retornou %d", len(items))
		}

		// Verificar se todos os itens estão presentes
		itemMap := make(map[string]bool)
		for _, item := range items {
			itemMap[item] = true
		}
		if !itemMap["all1"] || !itemMap["all2"] || !itemMap["all3"] {
			t.Error("GetAll não retornou todos os itens esperados")
		}
	})

	t.Run("Alterar TTL", func(t *testing.T) {
		es.Clear()
		
		// Definir novo TTL
		newTTL := 200 * time.Millisecond
		es.SetTTL(newTTL)
		
		// Adicionar item
		es.Add("ttlTest")
		
		// Esperar pelo TTL antigo
		time.Sleep(ttl + 10*time.Millisecond)
		
		// Item ainda deve existir
		if !es.Contains("ttlTest") {
			t.Error("ttlTest não deveria ter expirado com o novo TTL")
		}
		
		// Esperar pelo novo TTL
		time.Sleep(newTTL)
		
		// Item deve ter expirado
		if es.Contains("ttlTest") {
			t.Error("ttlTest deveria ter expirado após o novo TTL")
		}
	})
}
