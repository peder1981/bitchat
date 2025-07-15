// +build linux

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/permissionlesstech/bitchat/internal/bluetooth"
)

// LinuxPlatformProvider implementa PlatformProvider para Linux
type LinuxPlatformProvider struct {
	bluetoothAdapter bluetooth.LinuxBluetoothAdapter
	dataDir          string
	cacheDir         string
}

// newPlatformProvider retorna um provedor de plataforma para Linux
func newPlatformProvider() (PlatformProvider, error) {
	// Verificar se estamos no Linux
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("este provedor só é compatível com Linux, sistema atual: %s", runtime.GOOS)
	}

	// Diretórios de dados e cache
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("erro ao obter diretório home: %v", err)
	}

	dataDir := filepath.Join(homeDir, ".local", "share", "bitchat")
	cacheDir := filepath.Join(homeDir, ".cache", "bitchat")

	// Criar diretórios se não existirem
	for _, dir := range []string{dataDir, cacheDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("erro ao criar diretório %s: %v", dir, err)
		}
	}

	return &LinuxPlatformProvider{
		dataDir:  dataDir,
		cacheDir: cacheDir,
	}, nil
}

// GetBluetoothAdapter retorna o adaptador Bluetooth para Linux
func (lpp *LinuxPlatformProvider) GetBluetoothAdapter() BluetoothAdapter {
	// Implementação simplificada - em uma versão completa, inicializaríamos o adaptador aqui
	// e retornaríamos uma interface que encapsula LinuxBluetoothAdapter
	return nil
}

// GetMeshProvider retorna o provedor mesh para Linux
func (lpp *LinuxPlatformProvider) GetMeshProvider() MeshProvider {
	// Implementação simplificada - em uma versão completa, inicializaríamos o provedor mesh aqui
	return nil
}

// GetPlatformName retorna o nome da plataforma
func (lpp *LinuxPlatformProvider) GetPlatformName() string {
	return "Linux"
}

// GetPlatformVersion retorna a versão da plataforma
func (lpp *LinuxPlatformProvider) GetPlatformVersion() string {
	// Em uma implementação real, obteríamos a versão do kernel ou distribuição
	return runtime.GOOS + " " + runtime.GOARCH
}

// IsBatteryPowered retorna se o dispositivo é alimentado por bateria
func (lpp *LinuxPlatformProvider) IsBatteryPowered() bool {
	// Implementação simplificada - em uma versão completa, verificaríamos o sistema
	return false
}

// GetBatteryLevel retorna o nível de bateria
func (lpp *LinuxPlatformProvider) GetBatteryLevel() (int, error) {
	// Implementação simplificada
	return 0, fmt.Errorf("não implementado")
}

// GetDataDirectory retorna o diretório de dados
func (lpp *LinuxPlatformProvider) GetDataDirectory() string {
	return lpp.dataDir
}

// GetCacheDirectory retorna o diretório de cache
func (lpp *LinuxPlatformProvider) GetCacheDirectory() string {
	return lpp.cacheDir
}
