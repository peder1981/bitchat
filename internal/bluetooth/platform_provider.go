package bluetooth

import (
	"github.com/permissionlesstech/bitchat/internal/protocol"
)

// PlatformProvider define a interface para implementações específicas de plataforma
type PlatformProvider interface {
	// Initialize inicializa o provedor
	Initialize() error
	
	// Shutdown desliga o provedor
	Shutdown() error
	
	// SendPacket envia um pacote BitchatPacket
	SendPacket(packet *protocol.BitchatPacket) error
}

// NewPlatformProvider cria um novo provedor específico para a plataforma atual
func NewPlatformProvider(meshService *BluetoothMeshService) (PlatformProvider, error) {
	// Por enquanto, apenas o provedor Linux está implementado
	return NewLinuxMeshProvider(meshService)
}
