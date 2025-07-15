package platform

import (
	"context"

	"github.com/permissionlesstech/bitchat/internal/protocol"
)

// BluetoothAdapter define a interface comum para adaptadores Bluetooth específicos de plataforma
type BluetoothAdapter interface {
	// Inicialização e gerenciamento
	Initialize() error
	Start(ctx context.Context) error
	Stop() error
	IsRunning() bool
	
	// Configuração
	SetName(name string) error
	GetName() (string, error)
	SetDiscoverable(discoverable bool) error
	IsDiscoverable() (bool, error)
	
	// Descoberta e conexão
	StartDiscovery() error
	StopDiscovery() error
	IsDiscovering() (bool, error)
	GetDiscoveredDevices() ([]BluetoothDevice, error)
	
	// Anúncio e GATT
	StartAdvertising(serviceUUID string, manufacturerData []byte) error
	StopAdvertising() error
	IsAdvertising() (bool, error)
	
	// Serviço GATT
	RegisterGATTService(serviceUUID string, characteristicUUIDs []string) error
	UpdateCharacteristic(serviceUUID, characteristicUUID string, value []byte) error
	
	// Callbacks
	SetOnDeviceDiscoveredCallback(callback func(device BluetoothDevice))
	SetOnCharacteristicReadCallback(callback func(deviceID, serviceUUID, characteristicUUID string) []byte)
	SetOnCharacteristicWriteCallback(callback func(deviceID, serviceUUID, characteristicUUID string, value []byte))
	SetOnConnectionStateChangedCallback(callback func(deviceID string, connected bool))
	
	// Envio e recebimento de dados
	SendData(deviceID string, serviceUUID, characteristicUUID string, data []byte) error
	ReadCharacteristic(deviceID, serviceUUID, characteristicUUID string) ([]byte, error)
	
	// Informações do adaptador
	GetAdapterInfo() (BluetoothAdapterInfo, error)
}

// BluetoothDevice representa um dispositivo Bluetooth descoberto
type BluetoothDevice struct {
	ID          string
	Name        string
	Address     string
	RSSI        int
	Connected   bool
	ServiceData map[string][]byte
}

// BluetoothAdapterInfo contém informações sobre o adaptador Bluetooth local
type BluetoothAdapterInfo struct {
	Name    string
	Address string
	Powered bool
}

// MeshProvider define a interface para provedores de rede mesh específicos de plataforma
type MeshProvider interface {
	// Inicialização e gerenciamento
	Initialize() error
	Start(ctx context.Context) error
	Stop() error
	
	// Envio e recebimento de mensagens
	SendPacket(packet *protocol.BitchatPacket, targetPeerID string) error
	BroadcastPacket(packet *protocol.BitchatPacket) error
	
	// Gerenciamento de peers
	GetConnectedPeers() []string
	GetPeerSignalStrength(peerID string) int
	
	// Callbacks
	SetOnPacketReceivedCallback(callback func(packet *protocol.BitchatPacket, fromPeerID string))
	SetOnPeerDiscoveredCallback(callback func(peerID string, metadata map[string]string))
	SetOnPeerDisconnectedCallback(callback func(peerID string))
	
	// Configuração
	SetBatteryOptimizationEnabled(enabled bool)
	IsBatteryOptimizationEnabled() bool
	SetCoverTrafficEnabled(enabled bool)
	IsCoverTrafficEnabled() bool
}

// PlatformProvider é uma fábrica para criar implementações específicas de plataforma
type PlatformProvider interface {
	// Retorna o adaptador Bluetooth específico da plataforma
	GetBluetoothAdapter() BluetoothAdapter
	
	// Retorna o provedor mesh específico da plataforma
	GetMeshProvider() MeshProvider
	
	// Informações da plataforma
	GetPlatformName() string
	GetPlatformVersion() string
	IsBatteryPowered() bool
	GetBatteryLevel() (int, error)
	
	// Armazenamento específico da plataforma
	GetDataDirectory() string
	GetCacheDirectory() string
}

// NewPlatformProvider retorna o provedor de plataforma apropriado para o sistema operacional atual
func NewPlatformProvider() (PlatformProvider, error) {
	// Esta função será implementada em arquivos específicos de plataforma
	// usando build tags para selecionar a implementação correta
	return newPlatformProvider()
}
