//go:build linux
// +build linux

package bluetooth

import (
	"context"
	"fmt"
	
	"github.com/permissionlesstech/bitchat/internal/protocol"
)

// LinuxProvider implementa a interface PlatformProvider para Linux
type LinuxProvider struct {
	meshService *BluetoothMeshService
}

// NewPlatformProvider cria um novo provedor específico para Linux
func NewPlatformProvider(meshService *BluetoothMeshService) (PlatformProvider, error) {
	return &LinuxProvider{
		meshService: meshService,
	}, nil
}

// Initialize inicializa o provedor Linux
func (p *LinuxProvider) Initialize() error {
	// Implementação específica para Linux
	return nil
}

// Start inicia o provedor Linux
func (p *LinuxProvider) Start(ctx context.Context) error {
	// Implementação específica para Linux
	return nil
}

// Stop para o provedor Linux
func (p *LinuxProvider) Stop() error {
	// Implementação específica para Linux
	return nil
}

// SendPacket envia um pacote através do provedor Linux
func (p *LinuxProvider) SendPacket(packet *protocol.BitchatPacket) error {
	// Implementação específica para Linux
	return fmt.Errorf("envio de pacotes não implementado para Linux")
}
