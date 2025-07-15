//go:build darwin
// +build darwin

package bluetooth

import (
	"context"
	"fmt"
	
	"github.com/permissionlesstech/bitchat/internal/protocol"
)

// DarwinProvider implementa a interface PlatformProvider para macOS
type DarwinProvider struct {
	meshService *BluetoothMeshService
}

// NewPlatformProvider cria um novo provedor específico para macOS
func NewPlatformProvider(meshService *BluetoothMeshService) (PlatformProvider, error) {
	return nil, fmt.Errorf("provedor Bluetooth para macOS ainda não implementado")
}

// As funções abaixo não serão usadas, pois o provedor retorna erro na criação,
// mas são necessárias para satisfazer a interface caso a implementação seja adicionada no futuro

// Initialize inicializa o provedor macOS
func (p *DarwinProvider) Initialize() error {
	return fmt.Errorf("não implementado")
}

// Start inicia o provedor macOS
func (p *DarwinProvider) Start(ctx context.Context) error {
	return fmt.Errorf("não implementado")
}

// Stop para o provedor macOS
func (p *DarwinProvider) Stop() error {
	return fmt.Errorf("não implementado")
}

// SendPacket envia um pacote através do provedor macOS
func (p *DarwinProvider) SendPacket(packet *protocol.BitchatPacket) error {
	return fmt.Errorf("não implementado")
}
