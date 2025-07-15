// +build linux

package linux

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/permissionlesstech/bitchat/platform"
)

// LinuxPlatformProvider implementa a interface PlatformProvider para Linux
type LinuxPlatformProvider struct {
	bluetoothAdapter *LinuxBluetoothAdapter
	meshProvider     *LinuxMeshProvider
	dataDir          string
	cacheDir         string
}

// NewLinuxPlatformProvider cria uma nova instância do provedor de plataforma Linux
func NewLinuxPlatformProvider() (*LinuxPlatformProvider, error) {
	// Determinar diretórios de dados e cache
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	dataDir := filepath.Join(homeDir, ".local", "share", "bitchat")
	cacheDir := filepath.Join(homeDir, ".cache", "bitchat")

	// Criar diretórios se não existirem
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}

	// Criar adaptador Bluetooth
	bluetoothAdapter, err := NewLinuxBluetoothAdapter()
	if err != nil {
		return nil, err
	}

	// Criar provedor mesh
	meshProvider := NewLinuxMeshProvider(bluetoothAdapter)

	return &LinuxPlatformProvider{
		bluetoothAdapter: bluetoothAdapter,
		meshProvider:     meshProvider,
		dataDir:          dataDir,
		cacheDir:         cacheDir,
	}, nil
}

// GetBluetoothAdapter retorna o adaptador Bluetooth específico para Linux
func (p *LinuxPlatformProvider) GetBluetoothAdapter() platform.BluetoothAdapter {
	return p.bluetoothAdapter
}

// GetMeshProvider retorna o provedor mesh específico para Linux
func (p *LinuxPlatformProvider) GetMeshProvider() platform.MeshProvider {
	return p.meshProvider
}

// GetPlatformName retorna o nome da plataforma
func (p *LinuxPlatformProvider) GetPlatformName() string {
	return "Linux"
}

// GetPlatformVersion retorna a versão da plataforma
func (p *LinuxPlatformProvider) GetPlatformVersion() string {
	return runtime.GOOS + " " + runtime.GOARCH
}

// IsBatteryPowered verifica se o dispositivo é alimentado por bateria
func (p *LinuxPlatformProvider) IsBatteryPowered() bool {
	// Verificar se existe um diretório de bateria no sysfs
	batteryPath := "/sys/class/power_supply/BAT0"
	if _, err := os.Stat(batteryPath); err == nil {
		return true
	}
	
	// Verificar alternativa
	batteryPath = "/sys/class/power_supply/BAT1"
	if _, err := os.Stat(batteryPath); err == nil {
		return true
	}
	
	return false
}

// GetBatteryLevel retorna o nível de bateria atual (0-100)
func (p *LinuxPlatformProvider) GetBatteryLevel() (int, error) {
	// Tentar ler o nível de bateria do sysfs
	batteryPaths := []string{
		"/sys/class/power_supply/BAT0/capacity",
		"/sys/class/power_supply/BAT1/capacity",
	}
	
	for _, path := range batteryPaths {
		if data, err := os.ReadFile(path); err == nil {
			var level int
			if _, err := fmt.Sscanf(string(data), "%d", &level); err == nil {
				return level, nil
			}
		}
	}
	
	// Se não for possível ler o nível de bateria, retornar erro
	return 0, fmt.Errorf("não foi possível determinar o nível de bateria")
}

// GetDataDirectory retorna o diretório de dados da aplicação
func (p *LinuxPlatformProvider) GetDataDirectory() string {
	return p.dataDir
}

// GetCacheDirectory retorna o diretório de cache da aplicação
func (p *LinuxPlatformProvider) GetCacheDirectory() string {
	return p.cacheDir
}

// Função para ser chamada por NewPlatformProvider
func newPlatformProvider() (platform.PlatformProvider, error) {
	return NewLinuxPlatformProvider()
}
