package mesh

// GetNextHopCompat é uma função de compatibilidade para os testes de integração
// que aceita e retorna os tipos esperados pelos testes
func (r *MessageRouter) GetNextHopCompat(recipientID []byte) string {
	// Converter recipientID para string
	recipientIDStr := string(recipientID)
	
	// Chamar a implementação existente
	nextHop, _ := r.GetNextHop(recipientIDStr)
	return nextHop
}

// AddPeer adiciona um peer ao roteador (versão compatível com testes de integração)
func (r *MessageRouter) AddPeer(recipientID []byte) {
	// Converter recipientID para string
	recipientIDStr := string(recipientID)
	
	// Adicionar ao roteador
	r.UpdateRoutingInfo(recipientIDStr, recipientIDStr, 100)
}
