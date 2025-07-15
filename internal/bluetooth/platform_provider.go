package bluetooth

import (
	"context"
	
	"github.com/permissionlesstech/bitchat/internal/protocol"
)

// PlatformProvider define a interface para provedores de rede mesh específicos de plataforma
type PlatformProvider interface {
	// Inicialização e gerenciamento
	Initialize() error
	Start(ctx context.Context) error
	Stop() error
	
	// Envio e recebimento de mensagens
	SendPacket(packet *protocol.BitchatPacket) error
}

// NewPlatformProvider cria um novo provedor específico para a plataforma atual
// A implementação real é definida em cada plataforma usando build tags:
// - platform_provider_linux.go (Linux)
// - platform_provider_darwin.go (macOS)
// - platform_provider_windows.go (Windows)
