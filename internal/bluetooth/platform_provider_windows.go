//go:build windows
// +build windows

package bluetooth

import (
	"context"
	"fmt"
	
	"github.com/permissionlesstech/bitchat/internal/protocol"
)

// WindowsProvider implementa a interface PlatformProvider para Windows
type WindowsProvider struct {
	meshService *BluetoothMeshService
}

// NewPlatformProvider cria um novo provedor específico para Windows
func NewPlatformProvider(meshService *BluetoothMeshService) (PlatformProvider, error) {
	return nil, fmt.Errorf("provedor Bluetooth para Windows ainda não implementado")
}

// As funções abaixo não serão usadas, pois o provedor retorna erro na criação,
// mas são necessárias para satisfazer a interface caso a implementação seja adicionada no futuro

// Initialize inicializa o provedor Windows
func (p *WindowsProvider) Initialize() error {
	return fmt.Errorf("não implementado")
}

// Start inicia o provedor Windows
func (p *WindowsProvider) Start(ctx context.Context) error {
	return fmt.Errorf("não implementado")
}

// Stop para o provedor Windows
func (p *WindowsProvider) Stop() error {
	return fmt.Errorf("não implementado")
}

// SendPacket envia um pacote através do provedor Windows
func (p *WindowsProvider) SendPacket(packet *protocol.BitchatPacket) error {
	return fmt.Errorf("não implementado")
}
