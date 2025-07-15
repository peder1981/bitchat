package bluetooth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/muka/go-bluetooth/bluez/profile/advertising"
	"github.com/muka/go-bluetooth/bluez/profile/device"
)

// LinuxBluetoothAdapter implementa a funcionalidade BLE específica para Linux
type LinuxBluetoothAdapter struct {
	adapter           *adapter.Adapter1
	adMgr             *advertising.LEAdvertisingManager1
	advertisement     *advertising.LEAdvertisement1
	devices           map[string]*device.Device1
	deviceMutex       sync.RWMutex
	onDataReceived    func([]byte, string)
	ctx               context.Context
	cancel            context.CancelFunc
	isScanning        bool
	isAdvertising     bool
	cleanupAdvertisement func()
}

// NewLinuxBluetoothAdapter cria um novo adaptador BLE para Linux
func NewLinuxBluetoothAdapter() (*LinuxBluetoothAdapter, error) {
	// Obter adaptador padrão
	a, err := api.GetDefaultAdapter()
	if err != nil {
		return nil, fmt.Errorf("erro ao obter adaptador Bluetooth: %v", err)
	}

	// Verificar se o adaptador está ligado
	powered, err := a.GetPowered()
	if err != nil {
		return nil, fmt.Errorf("erro ao verificar estado do adaptador: %v", err)
	}

	if !powered {
		// Tentar ligar o adaptador
		if err := a.SetPowered(true); err != nil {
			return nil, fmt.Errorf("erro ao ligar adaptador Bluetooth: %v", err)
		}
	}

	// Obter gerenciador de advertising
	adMgr, err := advertising.NewLEAdvertisingManager1(a.Path())
	if err != nil {
		return nil, fmt.Errorf("erro ao criar gerenciador de advertising: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &LinuxBluetoothAdapter{
		adapter:     a,
		adMgr:       adMgr,
		devices:     make(map[string]*device.Device1),
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

// StartScanning inicia o escaneamento por dispositivos BLE
func (lba *LinuxBluetoothAdapter) StartScanning() error {
	if lba.isScanning {
		return nil
	}

	// Configurar filtro de descoberta
	filter := adapter.NewDiscoveryFilter()
	filter.Transport = "le"
	filter.UUIDs = []string{ServiceUUID}

	if err := lba.adapter.SetDiscoveryFilter(filter.ToMap()); err != nil {
		return fmt.Errorf("erro ao configurar filtro de descoberta: %v", err)
	}

	// Registrar callback para novos dispositivos
	discovery, cancel, err := api.Discover(lba.adapter, nil)
	if err != nil {
		return fmt.Errorf("erro ao iniciar descoberta: %v", err)
	}

	lba.isScanning = true

	// Processar dispositivos descobertos em goroutine
	go func() {
		defer cancel()

		for {
			select {
			case <-lba.ctx.Done():
				return
			case ev := <-discovery:
				if ev.Type == adapter.DeviceRemoved {
					lba.deviceMutex.Lock()
					delete(lba.devices, string(ev.Path))
					lba.deviceMutex.Unlock()
					continue
				}

				if ev.Type != adapter.DeviceAdded {
					continue
				}

				// Novo dispositivo encontrado
				dev, err := device.NewDevice1(ev.Path)
				if err != nil {
					fmt.Printf("Erro ao criar objeto de dispositivo: %v\n", err)
					continue
				}

				// Verificar se o dispositivo oferece o serviço Bitchat
				uuids, err := dev.GetUUIDs()
				if err != nil || !containsUUID(uuids, ServiceUUID) {
					continue
				}

				// Armazenar dispositivo
				lba.deviceMutex.Lock()
				lba.devices[string(ev.Path)] = dev
				lba.deviceMutex.Unlock()

				// Conectar ao dispositivo
				go lba.connectToDevice(dev)
			}
		}
	}()

	return nil
}

// StopScanning para o escaneamento por dispositivos
func (lba *LinuxBluetoothAdapter) StopScanning() error {
	if !lba.isScanning {
		return nil
	}

	if err := lba.adapter.StopDiscovery(); err != nil {
		return fmt.Errorf("erro ao parar descoberta: %v", err)
	}

	lba.isScanning = false
	return nil
}

// StartAdvertising inicia o advertising BLE
func (lba *LinuxBluetoothAdapter) StartAdvertising(deviceName string, serviceData []byte) error {
	if lba.isAdvertising {
		return nil
	}

	// Criar anúncio
	props := &advertising.LEAdvertisement1Properties{
		Type:      advertising.AdvertisementTypeBroadcast,
		ServiceUUIDs: []string{ServiceUUID},
		LocalName: deviceName,
		ServiceData: map[string]interface{}{
			ServiceUUID: serviceData,
		},
		Includes: []string{advertising.SupportedIncludesTxPower},
	}

	// Registrar anúncio usando ExposeAdvertisement
	adapterID, err := lba.adapter.GetAdapterID()
	if err != nil {
		return fmt.Errorf("erro ao obter ID do adaptador: %v", err)
	}
	cleanup, err := api.ExposeAdvertisement(adapterID, props, 0)
	if err != nil {
		return fmt.Errorf("erro ao criar anúncio: %v", err)
	}

	// Armazenar função de limpeza para uso posterior
	lba.cleanupAdvertisement = cleanup

	lba.isAdvertising = true

	return nil
}

// StopAdvertising para o advertising BLE
func (lba *LinuxBluetoothAdapter) StopAdvertising() error {
	if !lba.isAdvertising || lba.advertisement == nil {
		return nil
	}

	if err := lba.adMgr.UnregisterAdvertisement(lba.advertisement.Path()); err != nil {
		return fmt.Errorf("erro ao cancelar anúncio: %v", err)
	}

	lba.isAdvertising = false
	return nil
}

// SendData envia dados para um dispositivo específico
func (lba *LinuxBluetoothAdapter) SendData(data []byte, deviceID string) error {
	lba.deviceMutex.RLock()
	defer lba.deviceMutex.RUnlock()

	// Encontrar dispositivo pelo ID
	var targetDevice *device.Device1
	for _, dev := range lba.devices {
		addr, err := dev.GetAddress()
		if err == nil && addr == deviceID {
			targetDevice = dev
			break
		}
	}

	if targetDevice == nil {
		return fmt.Errorf("dispositivo não encontrado: %s", deviceID)
	}

	// Verificar se está conectado
	connected, err := targetDevice.GetConnected()
	if err != nil {
		return fmt.Errorf("erro ao verificar conexão: %v", err)
	}

	if !connected {
		// Tentar conectar
		if err := targetDevice.Connect(); err != nil {
			return fmt.Errorf("erro ao conectar ao dispositivo: %v", err)
		}

		// Aguardar conexão
		timeout := time.After(5 * time.Second)
		for {
			connected, err := targetDevice.GetConnected()
			if err != nil {
				return fmt.Errorf("erro ao verificar conexão: %v", err)
			}
			if connected {
				break
			}

			select {
			case <-timeout:
				return fmt.Errorf("timeout ao conectar ao dispositivo")
			case <-time.After(100 * time.Millisecond):
				// Continuar tentando
			}
		}
	}

	// Enviar dados (implementação depende da configuração GATT)
	// Esta é uma implementação simplificada; a real precisaria configurar
	// serviços GATT e características
	return fmt.Errorf("envio de dados não implementado completamente")
}

// BroadcastData envia dados para todos os dispositivos conectados
func (lba *LinuxBluetoothAdapter) BroadcastData(data []byte) error {
	lba.deviceMutex.RLock()
	defer lba.deviceMutex.RUnlock()

	var lastError error
	for _, dev := range lba.devices {
		addr, err := dev.GetAddress()
		if err != nil {
			continue
		}

		if err := lba.SendData(data, addr); err != nil {
			lastError = err
		}
	}

	return lastError
}

// SetOnDataReceived define o callback para dados recebidos
func (lba *LinuxBluetoothAdapter) SetOnDataReceived(callback func([]byte, string)) {
	lba.onDataReceived = callback
}

// Close libera recursos do adaptador
func (lba *LinuxBluetoothAdapter) Close() error {
	lba.cancel()

	// Parar advertising
	if lba.isAdvertising {
		lba.StopAdvertising()
	}

	// Parar escaneamento
	if lba.isScanning {
		lba.StopScanning()
	}

	// Desconectar dispositivos
	lba.deviceMutex.Lock()
	for _, dev := range lba.devices {
		dev.Disconnect()
	}
	lba.deviceMutex.Unlock()

	return nil
}

// Funções auxiliares

// connectToDevice conecta a um dispositivo e configura para receber dados
func (lba *LinuxBluetoothAdapter) connectToDevice(dev *device.Device1) {
	// Verificar se já está conectado
	connected, err := dev.GetConnected()
	if err != nil {
		fmt.Printf("Erro ao verificar conexão: %v\n", err)
		return
	}

	if !connected {
		// Tentar conectar
		if err := dev.Connect(); err != nil {
			fmt.Printf("Erro ao conectar ao dispositivo: %v\n", err)
			return
		}
	}

	// Configurar para receber notificações
	// Esta é uma implementação simplificada; a real precisaria descobrir
	// serviços e características GATT e configurar notificações
	fmt.Println("Dispositivo conectado, configuração para receber dados não implementada completamente")
}

// containsUUID verifica se uma lista contém um UUID específico
func containsUUID(uuids []string, target string) bool {
	for _, uuid := range uuids {
		if uuid == target {
			return true
		}
	}
	return false
}
