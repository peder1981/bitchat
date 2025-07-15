package bluetooth

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/permissionlesstech/bitchat/internal/protocol"
	"github.com/permissionlesstech/bitchat/pkg/utils"
)

// LinuxMeshProvider implementa a funcionalidade mesh BLE para Linux
type LinuxMeshProvider struct {
	adapter          *LinuxBluetoothAdapter
	meshService      *BluetoothMeshService
	fragmentManager  *FragmentManager
	mutex            sync.RWMutex
	isInitialized    bool
}

// NewLinuxMeshProvider cria um novo provedor mesh para Linux
func NewLinuxMeshProvider(meshService *BluetoothMeshService) (*LinuxMeshProvider, error) {
	adapter, err := NewLinuxBluetoothAdapter()
	if err != nil {
		return nil, fmt.Errorf("erro ao criar adaptador Bluetooth: %v", err)
	}

	provider := &LinuxMeshProvider{
		adapter:         adapter,
		meshService:     meshService,
		fragmentManager: NewFragmentManager(),
	}

	// Configurar callback para dados recebidos
	adapter.SetOnDataReceived(provider.handleReceivedData)

	return provider, nil
}

// Initialize inicializa o provedor mesh
func (lmp *LinuxMeshProvider) Initialize() error {
	lmp.mutex.Lock()
	defer lmp.mutex.Unlock()

	if lmp.isInitialized {
		return nil
	}

	// Iniciar escaneamento
	if err := lmp.adapter.StartScanning(); err != nil {
		return fmt.Errorf("erro ao iniciar escaneamento: %v", err)
	}

	// Iniciar advertising
	deviceName := lmp.meshService.deviceName
	
	// Dados do serviço para advertising (versão simplificada)
	serviceData := []byte{
		0x01, // Versão do protocolo
		byte(len(deviceName)),
	}
	serviceData = append(serviceData, []byte(deviceName)...)

	if err := lmp.adapter.StartAdvertising(deviceName, serviceData); err != nil {
		lmp.adapter.StopScanning()
		return fmt.Errorf("erro ao iniciar advertising: %v", err)
	}

	lmp.isInitialized = true
	return nil
}

// Shutdown desliga o provedor mesh
func (lmp *LinuxMeshProvider) Shutdown() error {
	lmp.mutex.Lock()
	defer lmp.mutex.Unlock()

	if !lmp.isInitialized {
		return nil
	}

	// Parar advertising e escaneamento
	lmp.adapter.StopAdvertising()
	lmp.adapter.StopScanning()
	
	// Fechar adaptador
	if err := lmp.adapter.Close(); err != nil {
		return fmt.Errorf("erro ao fechar adaptador: %v", err)
	}

	lmp.isInitialized = false
	return nil
}

// SendPacket envia um pacote BitchatPacket
func (lmp *LinuxMeshProvider) SendPacket(packet *protocol.BitchatPacket) error {
	// Codificar pacote
	data, err := protocol.Encode(packet)
	if err != nil {
		return fmt.Errorf("erro ao codificar pacote: %v", err)
	}

	// Verificar se precisa fragmentar
	if len(data) > MaxPacketSize {
		return lmp.sendFragmentedPacket(packet, data)
	}

	// Enviar pacote diretamente
	if isDirectedPacket(packet) {
		// Pacote direcionado para um peer específico
		recipientID := hex.EncodeToString(packet.RecipientID)
		return lmp.adapter.SendData(data, recipientID)
	} else {
		// Pacote broadcast
		return lmp.adapter.BroadcastData(data)
	}
}

// sendFragmentedPacket fragmenta e envia um pacote grande
func (lmp *LinuxMeshProvider) sendFragmentedPacket(packet *protocol.BitchatPacket, data []byte) error {
	// Gerar ID de fragmentação único
	fragmentID := utils.GenerateRandomID(4)
	
	// Calcular número de fragmentos
	numFragments := (len(data) + MaxFragmentPayloadSize - 1) / MaxFragmentPayloadSize
	
	// Criar e enviar fragmentos
	for i := 0; i < numFragments; i++ {
		// Determinar tipo de fragmento
		var fragType protocol.MessageType
		if i == 0 {
			fragType = protocol.MessageTypeFragmentStart
		} else if i == numFragments-1 {
			fragType = protocol.MessageTypeFragmentEnd
		} else {
			fragType = protocol.MessageTypeFragmentContinue
		}
		
		// Calcular offset e tamanho do fragmento
		offset := i * MaxFragmentPayloadSize
		end := offset + MaxFragmentPayloadSize
		if end > len(data) {
			end = len(data)
		}
		
		// Criar payload do fragmento
		fragPayload := make([]byte, 6+end-offset)
		copy(fragPayload[0:4], fragmentID)                  // ID do fragmento
		fragPayload[4] = byte(i)                            // Índice do fragmento
		fragPayload[5] = byte(numFragments)                 // Total de fragmentos
		copy(fragPayload[6:], data[offset:end])             // Dados do fragmento
		
		// Criar pacote de fragmento
		fragPacket := &protocol.BitchatPacket{
			Version:    packet.Version,
			Type:       fragType,
			SenderID:   packet.SenderID,
			RecipientID: packet.RecipientID,
			Timestamp:  packet.Timestamp,
			Payload:    fragPayload,
			TTL:        packet.TTL,
		}
		
		// Codificar e enviar fragmento
		fragData, err := protocol.Encode(fragPacket)
		if err != nil {
			return fmt.Errorf("erro ao codificar fragmento: %v", err)
		}
		
		if isDirectedPacket(packet) {
			recipientID := hex.EncodeToString(packet.RecipientID)
			if err := lmp.adapter.SendData(fragData, recipientID); err != nil {
				return err
			}
		} else {
			if err := lmp.adapter.BroadcastData(fragData); err != nil {
				return err
			}
		}
		
		// Pequena pausa entre fragmentos
		time.Sleep(20 * time.Millisecond)
	}
	
	return nil
}

// handleReceivedData processa dados recebidos do adaptador BLE
func (lmp *LinuxMeshProvider) handleReceivedData(data []byte, senderID string) {
	// Tentar decodificar pacote
	packet, err := protocol.Decode(data)
	if err != nil {
		fmt.Printf("Erro ao decodificar pacote: %v\n", err)
		return
	}
	
	// Verificar se é um fragmento
	if isFragmentPacket(packet) {
		lmp.handleFragmentPacket(packet, senderID)
		return
	}
	
	// Processar pacote normal
	lmp.meshService.incomingMessages <- packet
}

// handleFragmentPacket processa pacotes fragmentados
func (lmp *LinuxMeshProvider) handleFragmentPacket(packet *protocol.BitchatPacket, senderID string) {
	// Extrair informações do fragmento
	if len(packet.Payload) < 6 {
		fmt.Println("Fragmento inválido: payload muito pequeno")
		return
	}
	
	fragmentID := packet.Payload[0:4]
	fragmentIndex := int(packet.Payload[4])
	totalFragments := int(packet.Payload[5])
	fragmentData := packet.Payload[6:]
	
	// Adicionar fragmento ao gerenciador
	complete, reassembled := lmp.fragmentManager.AddFragment(
		fragmentID, 
		fragmentIndex, 
		totalFragments, 
		fragmentData,
		packet.Type == protocol.MessageTypeFragmentStart,
		packet.Type == protocol.MessageTypeFragmentEnd,
	)
	
	if complete {
		// Tentar decodificar pacote completo
		completePacket, err := protocol.Decode(reassembled)
		if err != nil {
			fmt.Printf("Erro ao decodificar pacote reassemblado: %v\n", err)
			return
		}
		
		// Enviar para processamento
		lmp.meshService.incomingMessages <- completePacket
	}
}

// Constantes e funções auxiliares

const (
	MaxPacketSize         = 512  // Tamanho máximo de pacote BLE
	MaxFragmentPayloadSize = 480  // Tamanho máximo de payload por fragmento
)

// isDirectedPacket verifica se um pacote é direcionado a um peer específico
func isDirectedPacket(packet *protocol.BitchatPacket) bool {
	if packet.RecipientID == nil || len(packet.RecipientID) == 0 {
		return false
	}
	
	// Verificar se é broadcast (todos 0xFF)
	for _, b := range packet.RecipientID {
		if b != 0xFF {
			return true
		}
	}
	
	return false
}

// isFragmentPacket verifica se um pacote é um fragmento
func isFragmentPacket(packet *protocol.BitchatPacket) bool {
	return packet.Type == protocol.MessageTypeFragmentStart ||
		packet.Type == protocol.MessageTypeFragmentContinue ||
		packet.Type == protocol.MessageTypeFragmentEnd
}

// FragmentManager gerencia a reassemblagem de pacotes fragmentados
type FragmentManager struct {
	fragments    map[string]map[int][]byte  // fragmentID -> index -> data
	startTime    map[string]time.Time       // fragmentID -> tempo de início
	totalFrags   map[string]int             // fragmentID -> total de fragmentos
	mutex        sync.Mutex
}

// NewFragmentManager cria um novo gerenciador de fragmentos
func NewFragmentManager() *FragmentManager {
	return &FragmentManager{
		fragments:  make(map[string]map[int][]byte),
		startTime:  make(map[string]time.Time),
		totalFrags: make(map[string]int),
	}
}

// AddFragment adiciona um fragmento e tenta reassemblar
// Retorna: completo, dados reassemblados
func (fm *FragmentManager) AddFragment(
	fragmentID []byte,
	index int,
	total int,
	data []byte,
	isStart bool,
	isEnd bool,
) (bool, []byte) {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()
	
	// Converter ID para string para usar como chave
	idStr := hex.EncodeToString(fragmentID)
	
	// Verificar se já temos este fragmento
	if _, exists := fm.fragments[idStr]; !exists {
		fm.fragments[idStr] = make(map[int][]byte)
		fm.startTime[idStr] = time.Now()
		fm.totalFrags[idStr] = total
	}
	
	// Armazenar fragmento
	fm.fragments[idStr][index] = data
	
	// Verificar se temos todos os fragmentos
	if len(fm.fragments[idStr]) == fm.totalFrags[idStr] {
		// Reassemblar pacote
		reassembled := fm.reassemblePacket(idStr)
		
		// Limpar dados deste fragmento
		delete(fm.fragments, idStr)
		delete(fm.startTime, idStr)
		delete(fm.totalFrags, idStr)
		
		return true, reassembled
	}
	
	// Limpar fragmentos antigos (mais de 30 segundos)
	fm.cleanupOldFragments()
	
	return false, nil
}

// reassemblePacket combina os fragmentos em um pacote completo
func (fm *FragmentManager) reassemblePacket(fragmentID string) []byte {
	fragments := fm.fragments[fragmentID]
	total := fm.totalFrags[fragmentID]
	
	// Calcular tamanho total
	totalSize := 0
	for i := 0; i < total; i++ {
		if frag, ok := fragments[i]; ok {
			totalSize += len(frag)
		}
	}
	
	// Combinar fragmentos
	result := make([]byte, 0, totalSize)
	for i := 0; i < total; i++ {
		if frag, ok := fragments[i]; ok {
			result = append(result, frag...)
		}
	}
	
	return result
}

// cleanupOldFragments remove fragmentos antigos
func (fm *FragmentManager) cleanupOldFragments() {
	now := time.Now()
	for id, startTime := range fm.startTime {
		if now.Sub(startTime) > 30*time.Second {
			delete(fm.fragments, id)
			delete(fm.startTime, id)
			delete(fm.totalFrags, id)
		}
	}
}
