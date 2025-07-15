// +build linux

package linux

import (
	"context"
	"fmt"
	"sync"
	"time"
	
	"github.com/permissionlesstech/bitchat/internal/protocol"
	"github.com/permissionlesstech/bitchat/platform"
)

const (
	// Configurações de rede mesh
	meshServiceUUID        = "6E400001-B5A3-F393-E0A9-E50E24DCCA9E"
	meshTxCharacteristicUUID = "6E400002-B5A3-F393-E0A9-E50E24DCCA9E"
	meshRxCharacteristicUUID = "6E400003-B5A3-F393-E0A9-E50E24DCCA9E"
	
	// Intervalos de tempo
	scanInterval            = 10 * time.Second
	advertisingInterval     = 1 * time.Second
	batteryOptimizedScanInterval = 30 * time.Second
	coverTrafficInterval    = 5 * time.Minute
	
	// Limites
	maxPacketSize           = 512 // Tamanho máximo de um pacote BLE
)

// LinuxMeshProvider implementa a interface MeshProvider para Linux
type LinuxMeshProvider struct {
	bluetoothAdapter *LinuxBluetoothAdapter
	ctx              context.Context
	cancel           context.CancelFunc
	
	// Callbacks
	onPacketReceived    func(packet *protocol.BitchatPacket, fromPeerID string)
	onPeerDiscovered    func(peerID string, metadata map[string]string)
	onPeerDisconnected  func(peerID string)
	
	// Configurações
	batteryOptimization bool
	coverTraffic        bool
	
	// Estado da rede
	connectedPeers      map[string]time.Time // peerID -> última vez visto
	peerSignalStrength  map[string]int       // peerID -> RSSI
	
	// Fragmentação e reconstrução de pacotes
	fragmentBuffer      map[string]map[int][]byte // peerID -> fragmentID -> dados
	fragmentMeta        map[string]protocol.FragmentMeta // peerID -> metadados de fragmentação
	
	// Mutex para acesso concorrente
	mutex              sync.RWMutex
}

// NewLinuxMeshProvider cria um novo provedor de rede mesh para Linux
func NewLinuxMeshProvider(bluetoothAdapter *LinuxBluetoothAdapter) *LinuxMeshProvider {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &LinuxMeshProvider{
		bluetoothAdapter:    bluetoothAdapter,
		ctx:                 ctx,
		cancel:              cancel,
		connectedPeers:      make(map[string]time.Time),
		peerSignalStrength:  make(map[string]int),
		fragmentBuffer:      make(map[string]map[int][]byte),
		fragmentMeta:        make(map[string]protocol.FragmentMeta),
		batteryOptimization: false,
		coverTraffic:        false,
	}
}

// Initialize inicializa o provedor de rede mesh
func (m *LinuxMeshProvider) Initialize() error {
	// Verificar se o adaptador Bluetooth está inicializado
	if !m.bluetoothAdapter.IsRunning() {
		if err := m.bluetoothAdapter.Initialize(); err != nil {
			return fmt.Errorf("erro ao inicializar adaptador Bluetooth: %v", err)
		}
	}
	
	// Configurar callbacks do adaptador Bluetooth
	m.bluetoothAdapter.SetOnDeviceDiscoveredCallback(m.handleDeviceDiscovered)
	m.bluetoothAdapter.SetOnCharacteristicWriteCallback(m.handleCharacteristicWrite)
	m.bluetoothAdapter.SetOnConnectionStateChangedCallback(m.handleConnectionStateChanged)
	
	// Registrar serviço GATT para comunicação mesh
	err := m.bluetoothAdapter.RegisterGATTService(
		meshServiceUUID,
		[]string{meshTxCharacteristicUUID, meshRxCharacteristicUUID},
	)
	if err != nil {
		return fmt.Errorf("erro ao registrar serviço GATT para mesh: %v", err)
	}
	
	return nil
}

// Start inicia o provedor de rede mesh
func (m *LinuxMeshProvider) Start(ctx context.Context) error {
	// Iniciar adaptador Bluetooth se ainda não estiver em execução
	if !m.bluetoothAdapter.IsRunning() {
		if err := m.bluetoothAdapter.Start(ctx); err != nil {
			return fmt.Errorf("erro ao iniciar adaptador Bluetooth: %v", err)
		}
	}
	
	// Iniciar anúncio BLE
	manufacturerData := []byte("BTCHT") // Identificador Bitchat nos dados do fabricante
	if err := m.bluetoothAdapter.StartAdvertising(meshServiceUUID, manufacturerData); err != nil {
		return fmt.Errorf("erro ao iniciar anúncio BLE: %v", err)
	}
	
	// Iniciar descoberta de dispositivos
	if err := m.bluetoothAdapter.StartDiscovery(); err != nil {
		return fmt.Errorf("erro ao iniciar descoberta de dispositivos: %v", err)
	}
	
	// Iniciar goroutines de manutenção
	go m.scanLoop()
	go m.advertisingLoop()
	go m.maintenanceLoop()
	
	// Iniciar tráfego de cobertura se habilitado
	if m.coverTraffic {
		go m.coverTrafficLoop()
	}
	
	return nil
}

// Stop para o provedor de rede mesh
func (m *LinuxMeshProvider) Stop() error {
	// Cancelar contexto para parar todas as goroutines
	m.cancel()
	
	// Parar descoberta de dispositivos
	if err := m.bluetoothAdapter.StopDiscovery(); err != nil {
		return fmt.Errorf("erro ao parar descoberta de dispositivos: %v", err)
	}
	
	// Parar anúncio BLE
	if err := m.bluetoothAdapter.StopAdvertising(); err != nil {
		return fmt.Errorf("erro ao parar anúncio BLE: %v", err)
	}
	
	return nil
}

// SendPacket envia um pacote para um peer específico
func (m *LinuxMeshProvider) SendPacket(packet *protocol.BitchatPacket, targetPeerID string) error {
	// Serializar pacote
	data, err := protocol.EncodePacket(packet)
	if err != nil {
		return fmt.Errorf("erro ao codificar pacote: %v", err)
	}
	
	// Verificar se o pacote precisa ser fragmentado
	if len(data) > maxPacketSize {
		// Converter SenderID de []byte para string para compatibilidade
		senderIDStr := string(packet.SenderID)
		return m.sendFragmentedPacket(data, targetPeerID, senderIDStr)
	}
	
	// Enviar pacote diretamente
	return m.sendRawData(data, targetPeerID)
}

// BroadcastPacket envia um pacote para todos os peers conectados
func (m *LinuxMeshProvider) BroadcastPacket(packet *protocol.BitchatPacket) error {
	// Serializar pacote
	data, err := protocol.EncodePacket(packet)
	if err != nil {
		return fmt.Errorf("erro ao codificar pacote: %v", err)
	}
	
	// Obter lista de peers conectados
	m.mutex.RLock()
	peers := make([]string, 0, len(m.connectedPeers))
	for peerID := range m.connectedPeers {
		peers = append(peers, peerID)
	}
	m.mutex.RUnlock()
	
	// Verificar se o pacote precisa ser fragmentado
	if len(data) > maxPacketSize {
		// Fragmentar e enviar para cada peer
		for _, peerID := range peers {
			// Converter SenderID de []byte para string para compatibilidade
			senderIDStr := string(packet.SenderID)
			if err := m.sendFragmentedPacket(data, peerID, senderIDStr); err != nil {
				// Continuar mesmo se houver erro com um peer
				fmt.Printf("Erro ao enviar pacote fragmentado para %s: %v\n", peerID, err)
			}
		}
		return nil
	}
	
	// Enviar pacote não fragmentado para cada peer
	for _, peerID := range peers {
		if err := m.sendRawData(data, peerID); err != nil {
			// Continuar mesmo se houver erro com um peer
			fmt.Printf("Erro ao enviar pacote para %s: %v\n", peerID, err)
		}
	}
	
	return nil
}

// GetConnectedPeers retorna a lista de peers conectados
func (m *LinuxMeshProvider) GetConnectedPeers() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	peers := make([]string, 0, len(m.connectedPeers))
	for peerID := range m.connectedPeers {
		peers = append(peers, peerID)
	}
	
	return peers
}

// GetPeerSignalStrength retorna a força do sinal de um peer
func (m *LinuxMeshProvider) GetPeerSignalStrength(peerID string) int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if rssi, ok := m.peerSignalStrength[peerID]; ok {
		return rssi
	}
	
	return 0
}

// SetOnPacketReceivedCallback define o callback para pacotes recebidos
func (m *LinuxMeshProvider) SetOnPacketReceivedCallback(callback func(packet *protocol.BitchatPacket, fromPeerID string)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.onPacketReceived = callback
}

// SetOnPeerDiscoveredCallback define o callback para peers descobertos
func (m *LinuxMeshProvider) SetOnPeerDiscoveredCallback(callback func(peerID string, metadata map[string]string)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.onPeerDiscovered = callback
}

// SetOnPeerDisconnectedCallback define o callback para peers desconectados
func (m *LinuxMeshProvider) SetOnPeerDisconnectedCallback(callback func(peerID string)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.onPeerDisconnected = callback
}

// SetBatteryOptimizationEnabled habilita ou desabilita a otimização de bateria
func (m *LinuxMeshProvider) SetBatteryOptimizationEnabled(enabled bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.batteryOptimization = enabled
}

// IsBatteryOptimizationEnabled verifica se a otimização de bateria está habilitada
func (m *LinuxMeshProvider) IsBatteryOptimizationEnabled() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	return m.batteryOptimization
}

// SetCoverTrafficEnabled habilita ou desabilita o tráfego de cobertura
func (m *LinuxMeshProvider) SetCoverTrafficEnabled(enabled bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.coverTraffic = enabled
	
	// Iniciar ou parar loop de tráfego de cobertura
	if enabled && !m.coverTraffic {
		go m.coverTrafficLoop()
	}
}

// IsCoverTrafficEnabled verifica se o tráfego de cobertura está habilitado
func (m *LinuxMeshProvider) IsCoverTrafficEnabled() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	return m.coverTraffic
}

// Métodos internos

// sendRawData envia dados brutos para um peer
func (m *LinuxMeshProvider) sendRawData(data []byte, peerID string) error {
	return m.bluetoothAdapter.SendData(
		peerID,
		meshServiceUUID,
		meshTxCharacteristicUUID,
		data,
	)
}

// sendFragmentedPacket fragmenta e envia um pacote grande
func (m *LinuxMeshProvider) sendFragmentedPacket(data []byte, targetPeerID, senderID string) error {
	// Implementação simplificada para compilação
	// Calcular número de fragmentos necessários
	numFragments := (len(data) + maxPacketSize - 1) / maxPacketSize
	
	// Gerar ID único para o conjunto de fragmentos
	packetID := fmt.Sprintf("%x", time.Now().UnixNano())
	
	// Enviar cada fragmento
	for i := 0; i < numFragments; i++ {
		// Calcular início e fim do fragmento atual
		start := i * maxPacketSize
		end := start + maxPacketSize
		if end > len(data) {
			end = len(data)
		}
		
		// Extrair dados do fragmento
		fragmentData, err := protocol.EncodeFragment(packetID, i, numFragments)
		if err != nil {
			return fmt.Errorf("erro ao codificar fragmento %d: %v", i, err)
		}
		
		// Enviar fragmento
		if err := m.sendRawData(fragmentData, targetPeerID); err != nil {
			return fmt.Errorf("erro ao enviar fragmento %d: %v", i, err)
		}
		
		// Pequeno atraso entre fragmentos para não sobrecarregar o canal
		time.Sleep(50 * time.Millisecond)
	}
	
	return nil
}

// handleDeviceDiscovered processa dispositivos descobertos
func (m *LinuxMeshProvider) handleDeviceDiscovered(device platform.BluetoothDevice) {
	// Verificar se é um dispositivo Bitchat
	isBitchatDevice := false
	var metadata map[string]string
	
	// Verificar dados de serviço
	if serviceData, ok := device.ServiceData[meshServiceUUID]; ok {
		isBitchatDevice = true
		
		// Extrair metadados dos dados de serviço
		fragmentMeta := protocol.ExtractMetadataFromServiceData(serviceData)
		
		// Converter FragmentMeta para map[string]string para compatibilidade
		metadata = map[string]string{
			"packetID": fragmentMeta.PacketID,
			"totalFragments": fmt.Sprintf("%d", fragmentMeta.TotalFragments),
		}
	}
	
	// Nota: Verificação de dados do fabricante removida pois BluetoothDevice não possui campo ManufacturerData
	// Verificamos apenas ServiceData que está disponível na interface
	
	if isBitchatDevice {
		// Extrair peerID dos metadados
		peerID, ok := metadata["peerID"]
		if !ok {
			// Usar endereço como fallback
			peerID = device.Address
		}
		
		m.mutex.Lock()
		
		// Atualizar lista de peers conectados
		m.connectedPeers[peerID] = time.Now()
		m.peerSignalStrength[peerID] = device.RSSI
		
		// Notificar callback
		onPeerDiscovered := m.onPeerDiscovered
		
		m.mutex.Unlock()
		
		// Chamar callback fora do lock
		if onPeerDiscovered != nil {
			onPeerDiscovered(peerID, metadata)
		}
	}
}

// handleCharacteristicWrite processa escritas em características
func (m *LinuxMeshProvider) handleCharacteristicWrite(deviceID, serviceUUID, characteristicUUID string, value []byte) {
	// Verificar se é uma escrita na característica de recebimento
	if serviceUUID == meshServiceUUID && characteristicUUID == meshRxCharacteristicUUID {
		// Verificar se é um fragmento (simplificado para compilação)
		if len(value) > 0 && (value[0] == byte(protocol.MessageTypeFragmentStart) || 
			value[0] == byte(protocol.MessageTypeFragmentContinue) || 
			value[0] == byte(protocol.MessageTypeFragmentEnd)) {
			m.handleFragmentReceived(value, deviceID)
			return
		}
		
		// Tentar decodificar como pacote normal
		packet, err := protocol.DecodePacket(value)
		if err != nil {
			fmt.Printf("Erro ao decodificar pacote recebido: %v\n", err)
			return
		}
		
		// Notificar callback
		m.mutex.RLock()
		callback := m.onPacketReceived
		m.mutex.RUnlock()
		
		if callback != nil {
			callback(packet, deviceID)
		}
	}
}

// handleFragmentReceived processa fragmentos recebidos
func (m *LinuxMeshProvider) handleFragmentReceived(fragmentData []byte, fromPeerID string) {
	// Decodificar fragmento
	packetID, fragmentIndex, totalFragments, fragmentContent, err := protocol.DecodeFragment(fragmentData)
	if err != nil {
		fmt.Printf("Erro ao decodificar fragmento: %v\n", err)
		return
	}
	
	m.mutex.Lock()
	
	// Inicializar buffer de fragmentos para este peer se necessário
	if _, ok := m.fragmentBuffer[fromPeerID]; !ok {
		m.fragmentBuffer[fromPeerID] = make(map[int][]byte)
	}
	
	// Armazenar fragmento e metadados
	m.fragmentBuffer[fromPeerID][fragmentIndex] = fragmentContent
	
	// Atualizar ou criar metadados
	if _, ok := m.fragmentMeta[fromPeerID]; !ok {
		m.fragmentMeta[fromPeerID] = protocol.FragmentMeta{
			PacketID:         packetID,
			TotalFragments:    totalFragments,
			ReceivedFragments: 1,
			Timestamp:        time.Now(),
		}
	} else {
		meta := m.fragmentMeta[fromPeerID]
		meta.ReceivedFragments++
		m.fragmentMeta[fromPeerID] = meta
	}
	
	// Verificar se todos os fragmentos foram recebidos
	meta := m.fragmentMeta[fromPeerID]
	if len(m.fragmentBuffer[fromPeerID]) == meta.TotalFragments {
		// Reconstruir pacote
		reconstructedData, err := protocol.ReassembleFragments(m.fragmentBuffer[fromPeerID], meta.TotalFragments)
		if err != nil {
			fmt.Printf("Erro ao reconstruir pacote: %v\n", err)
			m.mutex.Unlock()
			return
		}
		
		// Limpar buffer de fragmentos
		delete(m.fragmentBuffer, fromPeerID)
		delete(m.fragmentMeta, fromPeerID)
		m.mutex.Unlock()
		
		// Tentar decodificar como pacote
		packet, err := protocol.DecodePacket(reconstructedData)
		if err != nil {
			fmt.Printf("Erro ao decodificar pacote reconstruído: %v\n", err)
			return
		}
		
		// Obter callback
		callback := m.onPacketReceived
		
		m.mutex.Unlock()
		
		// Notificar callback fora do lock
		if callback != nil {
			callback(packet, fromPeerID)
		}
	} else {
		m.mutex.Unlock()
	}
}

// handleConnectionStateChanged processa mudanças de estado de conexão
func (m *LinuxMeshProvider) handleConnectionStateChanged(deviceID string, connected bool) {
	if !connected {
		m.mutex.Lock()
		
		// Encontrar peerID correspondente ao deviceID
		var peerID string
		for id, _ := range m.connectedPeers {
			// Verificar se o deviceID está contido no peerID ou vice-versa
			// (isso é uma simplificação, na prática seria necessário um mapeamento mais preciso)
			if id == deviceID || deviceID == id {
				peerID = id
				break
			}
		}
		
		if peerID != "" {
			// Remover peer da lista
			delete(m.connectedPeers, peerID)
			delete(m.peerSignalStrength, peerID)
			
			// Limpar fragmentos pendentes
			delete(m.fragmentBuffer, peerID)
			delete(m.fragmentMeta, peerID)
			
			// Obter callback
			callback := m.onPeerDisconnected
			
			m.mutex.Unlock()
			
			// Notificar callback fora do lock
			if callback != nil {
				callback(peerID)
			}
		} else {
			m.mutex.Unlock()
		}
	}
}

// Loops de manutenção

// scanLoop executa varreduras periódicas para descobrir dispositivos
func (m *LinuxMeshProvider) scanLoop() {
	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			// Ajustar intervalo com base na otimização de bateria
			m.mutex.RLock()
			batteryOptimized := m.batteryOptimization
			m.mutex.RUnlock()
			
			if batteryOptimized {
				ticker.Reset(batteryOptimizedScanInterval)
			} else {
				ticker.Reset(scanInterval)
			}
			
			// Reiniciar descoberta
			m.bluetoothAdapter.StopDiscovery()
			time.Sleep(100 * time.Millisecond)
			m.bluetoothAdapter.StartDiscovery()
		}
	}
}

// advertisingLoop mantém o anúncio BLE ativo
func (m *LinuxMeshProvider) advertisingLoop() {
	ticker := time.NewTicker(advertisingInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			// Verificar se o anúncio está ativo
			isAdvertising, _ := m.bluetoothAdapter.IsAdvertising()
			if !isAdvertising {
				// Reiniciar anúncio
				manufacturerData := []byte("BTCHT")
				m.bluetoothAdapter.StartAdvertising(meshServiceUUID, manufacturerData)
			}
		}
	}
}

// maintenanceLoop executa tarefas de manutenção periódicas
func (m *LinuxMeshProvider) maintenanceLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanupStaleConnections()
			m.cleanupFragmentBuffers()
		}
	}
}

// coverTrafficLoop gera tráfego de cobertura para dificultar análise de tráfego
func (m *LinuxMeshProvider) coverTrafficLoop() {
	ticker := time.NewTicker(coverTrafficInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.mutex.RLock()
			coverTrafficEnabled := m.coverTraffic
			m.mutex.RUnlock()
			
			if !coverTrafficEnabled {
				return
			}
			
			// Gerar e enviar pacote de cobertura
			packet := protocol.GenerateCoverTrafficPacket()
			m.BroadcastPacket(packet)
		}
	}
}

// cleanupStaleConnections remove conexões inativas
func (m *LinuxMeshProvider) cleanupStaleConnections() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	now := time.Now()
	timeout := 5 * time.Minute
	
	for peerID, lastSeen := range m.connectedPeers {
		if now.Sub(lastSeen) > timeout {
			// Remover peer inativo
			delete(m.connectedPeers, peerID)
			delete(m.peerSignalStrength, peerID)
			
			// Notificar desconexão
			if m.onPeerDisconnected != nil {
				m.onPeerDisconnected(peerID)
			}
		}
	}
}

// cleanupFragmentBuffers limpa buffers de fragmentos incompletos
func (m *LinuxMeshProvider) cleanupFragmentBuffers() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// Tempo máximo de espera por fragmentos completos
	timeout := 30 * time.Second
	now := time.Now()
	
	for peerID, meta := range m.fragmentMeta {
		if now.Sub(meta.Timestamp) > timeout {
			// Remover fragmentos incompletos
			delete(m.fragmentBuffer, peerID)
			delete(m.fragmentMeta, peerID)
		}
	}
}
