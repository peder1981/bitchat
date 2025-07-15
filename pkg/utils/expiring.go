package utils

import (
	"sync"
	"time"
)

// ExpiringSet é um conjunto que automaticamente remove itens após um período de tempo
// Útil para deduplicação de mensagens e cache com TTL
type ExpiringSet struct {
	items    map[string]time.Time
	mutex    sync.RWMutex
	ttl      time.Duration
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewExpiringSet cria um novo conjunto com expiração
// ttl: tempo de vida dos itens
// cleanupInterval: intervalo para verificar e remover itens expirados
func NewExpiringSet(ttl time.Duration, cleanupInterval time.Duration) *ExpiringSet {
	es := &ExpiringSet{
		items:    make(map[string]time.Time),
		ttl:      ttl,
		stopChan: make(chan struct{}),
	}

	// Iniciar goroutine de limpeza
	es.wg.Add(1)
	go func() {
		defer es.wg.Done()
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				es.cleanup()
			case <-es.stopChan:
				return
			}
		}
	}()

	return es
}

// Add adiciona um item ao conjunto
// Retorna true se o item foi adicionado, false se já existia
func (es *ExpiringSet) Add(item string) bool {
	es.mutex.Lock()
	defer es.mutex.Unlock()

	now := time.Now()
	if expiry, exists := es.items[item]; exists && expiry.After(now) {
		// Item já existe e não expirou
		return false
	}

	// Adicionar ou atualizar item
	es.items[item] = now.Add(es.ttl)
	return true
}

// Contains verifica se um item está no conjunto
func (es *ExpiringSet) Contains(item string) bool {
	es.mutex.RLock()
	defer es.mutex.RUnlock()

	expiry, exists := es.items[item]
	return exists && expiry.After(time.Now())
}

// Remove remove um item do conjunto
func (es *ExpiringSet) Remove(item string) {
	es.mutex.Lock()
	defer es.mutex.Unlock()

	delete(es.items, item)
}

// Size retorna o número de itens no conjunto
func (es *ExpiringSet) Size() int {
	es.mutex.RLock()
	defer es.mutex.RUnlock()

	count := 0
	now := time.Now()
	for _, expiry := range es.items {
		if expiry.After(now) {
			count++
		}
	}
	return count
}

// Clear remove todos os itens do conjunto
func (es *ExpiringSet) Clear() {
	es.mutex.Lock()
	defer es.mutex.Unlock()

	es.items = make(map[string]time.Time)
}

// Stop interrompe a goroutine de limpeza
func (es *ExpiringSet) Stop() {
	close(es.stopChan)
	es.wg.Wait()
}

// cleanup remove itens expirados do conjunto
func (es *ExpiringSet) cleanup() {
	es.mutex.Lock()
	defer es.mutex.Unlock()

	now := time.Now()
	for item, expiry := range es.items {
		if expiry.Before(now) {
			delete(es.items, item)
		}
	}
}

// GetAll retorna todos os itens não expirados no conjunto
func (es *ExpiringSet) GetAll() []string {
	es.mutex.RLock()
	defer es.mutex.RUnlock()

	result := make([]string, 0, len(es.items))
	now := time.Now()
	
	for item, expiry := range es.items {
		if expiry.After(now) {
			result = append(result, item)
		}
	}
	
	return result
}

// SetTTL altera o tempo de vida dos itens
// Nota: isso não afeta itens já adicionados
func (es *ExpiringSet) SetTTL(ttl time.Duration) {
	es.mutex.Lock()
	defer es.mutex.Unlock()
	
	es.ttl = ttl
}

// UpdateExpiry atualiza o tempo de expiração de um item
// Retorna true se o item foi atualizado, false se não existia
func (es *ExpiringSet) UpdateExpiry(item string) bool {
	es.mutex.Lock()
	defer es.mutex.Unlock()
	
	if _, exists := es.items[item]; !exists {
		return false
	}
	
	es.items[item] = time.Now().Add(es.ttl)
	return true
}
