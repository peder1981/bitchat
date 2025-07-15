package bluetooth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/permissionlesstech/bitchat/internal/protocol"
	"github.com/permissionlesstech/bitchat/internal/crypto"
	"github.com/permissionlesstech/bitchat/pkg/utils"
)

const (
	// Constantes para o serviço BLE
	ServiceUUID        = "6E400001-B5A3-F393-E0A9-E50E24DCCA9E" // UUID do serviço Bitchat
	CharacteristicUUID = "6E400002-B5A3-F393-E0A9-E50E24DCCA9E" // UUID da característica de dados
	
	// Configurações de operação
	DefaultScanInterval    = 10 * time.Second
	DefaultAdvertiseInterval = 5 * time.Second
	DefaultMessageCacheTTL = 5 * time.Minute
	DefaultMessageCacheSize = 1000
	
	// Modos de economia de bateria
	BatteryModeNormal      = 0
	BatteryModeLow         = 1
	BatteryModeUltraLow    = 2
)

// Erros do serviço Bluetooth Mesh
var (
	ErrBluetoothNotAvailable = errors.New("bluetooth não disponível")
	ErrSendFailed            = errors.New("falha ao enviar mensagem")
	ErrInvalidPacket         = errors.New("pacote inválido")
	ErrPeerNotFound          = errors.New("peer não encontrado")
)

// MeshDelegate é a interface para receber eventos do serviço mesh
type MeshDelegate interface {
	OnPeerDiscovered(peerID string, name string)
	OnPeerLost(peerID string)
	OnMessageReceived(message *protocol.BitchatMessage)
	OnMessageDeliveryChanged(messageID string, status protocol.DeliveryStatus, info *protocol.DeliveryInfo)
}

// BluetoothMeshService gerencia a rede mesh Bluetooth
type BluetoothMeshService struct {
	// Identificação
	deviceID        []byte
	deviceName      string
	
	// Dependências
	encryptionService *crypto.EncryptionService
	delegate          MeshDelegate
	platformProvider  PlatformProvider
	
	// Estado da rede mesh
	peers            map[string]*Peer
	messageCache     *MessageCache
	seenMessages     *utils.ExpiringSet
	
	// Configurações
	batteryMode      int
	coverTraffic     bool
	
	// Controle de operação
	ctx              context.Context
	cancel           context.CancelFunc
	mutex            sync.RWMutex
	isRunning        bool
	
	// Canais para comunicação interna
	outgoingMessages chan *protocol.BitchatPacket
	incomingMessages chan *protocol.BitchatPacket
}

// Peer representa um dispositivo na rede mesh
type Peer struct {
	ID              string
	Name            string
	LastSeen        time.Time
	PublicKeyData   []byte
	RSSI            int
	HopCount        int
	IsRelay         bool
	MessageQueue    []*protocol.BitchatPacket
}

// MessageCache implementa cache para store-and-forward
type MessageCache struct {
	messages        map[string]*CachedMessage
	maxSize         int
	mutex           sync.RWMutex
}

// CachedMessage armazena uma mensagem em cache com metadados
type CachedMessage struct {
	Packet          *protocol.BitchatPacket
	ReceivedAt      time.Time
	ExpiresAt       time.Time
	DeliveredTo     map[string]bool
	OriginalSender  string
}

// NewBluetoothMeshService cria um novo serviço mesh Bluetooth
func NewBluetoothMeshService(
	deviceID []byte,
	deviceName string,
	encryptionService *crypto.EncryptionService,
) *BluetoothMeshService {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &BluetoothMeshService{
		deviceID:         deviceID,
		deviceName:       deviceName,
		encryptionService: encryptionService,
		peers:            make(map[string]*Peer),
		messageCache:     newMessageCache(DefaultMessageCacheSize),
		seenMessages:     utils.NewExpiringSet(DefaultMessageCacheTTL, DefaultMessageCacheTTL),
		batteryMode:      BatteryModeNormal,
		coverTraffic:     true,
		ctx:              ctx,
		cancel:           cancel,
		outgoingMessages: make(chan *protocol.BitchatPacket, 100),
		incomingMessages: make(chan *protocol.BitchatPacket, 100),
	}
}

// newMessageCache cria um novo cache de mensagens
func newMessageCache(maxSize int) *MessageCache {
	return &MessageCache{
		messages: make(map[string]*CachedMessage),
		maxSize:  maxSize,
	}
}

// SetDelegate define o delegate para receber eventos
func (bms *BluetoothMeshService) SetDelegate(delegate MeshDelegate) {
	bms.delegate = delegate
}

// Start inicia o serviço Bluetooth mesh
func (bms *BluetoothMeshService) Start() error {
	bms.mutex.Lock()
	defer bms.mutex.Unlock()
	
	if bms.isRunning {
		return nil
	}
	
	// Criar provedor específico da plataforma se ainda não existir
	if bms.platformProvider == nil {
		provider, err := NewPlatformProvider(bms)
		if err != nil {
			return fmt.Errorf("erro ao criar provedor de plataforma: %v", err)
		}
		bms.platformProvider = provider
	}
	
	// Inicializar provedor de plataforma
	if err := bms.platformProvider.Initialize(); err != nil {
		return fmt.Errorf("erro ao inicializar provedor de plataforma: %v", err)
	}
	
	// Iniciar goroutines
	go bms.maintenanceLoop()
	go bms.processOutgoingMessages()
	go bms.processIncomingMessages()
	
	bms.isRunning = true
	fmt.Println("Serviço Bluetooth mesh iniciado com sucesso")
	return nil
}

// Stop para o serviço Bluetooth mesh
func (bms *BluetoothMeshService) Stop() {
	bms.mutex.Lock()
	defer bms.mutex.Unlock()
	
	if !bms.isRunning {
		return
	}
	
	// Parar provedor de plataforma
	if bms.platformProvider != nil {
		if err := bms.platformProvider.Stop(); err != nil {
			fmt.Printf("Erro ao desligar provedor de plataforma: %v\n", err)
		}
	}
	
	// Parar goroutines
	bms.cancel()
	
	// Criar novo contexto para próximo início
	ctx, cancel := context.WithCancel(context.Background())
	bms.ctx = ctx
	bms.cancel = cancel
	
	bms.isRunning = false
	fmt.Println("Serviço Bluetooth mesh parado")
}

// SendMessage envia uma mensagem através da rede mesh
func (bms *BluetoothMeshService) SendMessage(message *protocol.BitchatMessage) (string, error) {
	// Criar pacote a partir da mensagem
	packet := &protocol.BitchatPacket{
		Version:    1,
		Type:       protocol.MessageTypeMessage,
		SenderID:   bms.deviceID,
		Timestamp:  uint64(time.Now().UnixMilli()),
		TTL:        7, // Valor padrão para TTL
	}
	
	// Definir destinatário
	if message.IsPrivate {
		// Buscar peer pelo nickname
		peerID := bms.findPeerIDByNickname(message.RecipientNickname)
		if peerID == "" {
			return "", ErrPeerNotFound
		}
		
		// Criptografar conteúdo para mensagem privada
		encryptedContent, _, err := bms.encryptionService.Encrypt([]byte(message.Content), []byte(peerID))
		if err != nil {
			return "", err
		}
		
		packet.RecipientID = []byte(peerID)
		packet.Payload = encryptedContent
		message.EncryptedContent = encryptedContent
		message.IsEncrypted = true
	} else if message.Channel != "" {
		// Mensagem de canal (broadcast com criptografia de canal)
		// Implementação completa requer serviço de canal
		packet.RecipientID = protocol.BroadcastRecipient
		packet.Payload = []byte(message.Content)
	} else {
		// Broadcast simples
		packet.RecipientID = protocol.BroadcastRecipient
		packet.Payload = []byte(message.Content)
	}
	
	// Assinar pacote
	signature, err := bms.encryptionService.Sign(packet.Payload)
	if err != nil {
		return "", fmt.Errorf("erro ao assinar pacote: %w", err)
	}
	packet.Signature = signature
	
	// Gerar ID de mensagem
	messageID := utils.GenerateMessageID(packet)
	message.ID = messageID
	
	// Enviar para processamento
	bms.outgoingMessages <- packet
	
	return messageID, nil
}

// SetBatteryMode define o modo de economia de bateria
func (bms *BluetoothMeshService) SetBatteryMode(mode int) {
	bms.mutex.Lock()
	defer bms.mutex.Unlock()
	
	bms.batteryMode = mode
}

// SetCoverTraffic ativa ou desativa o tráfego de cobertura
func (bms *BluetoothMeshService) SetCoverTraffic(enabled bool) {
	bms.mutex.Lock()
	defer bms.mutex.Unlock()
	
	bms.coverTraffic = enabled
}

// maintenanceLoop executa tarefas periódicas de manutenção
func (bms *BluetoothMeshService) maintenanceLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-bms.ctx.Done():
			return
		case <-ticker.C:
			// Limpar mensagens expiradas do cache
			bms.cleanupExpiredMessages()
			
			// Remover peers inativos
			bms.cleanupInactivePeers()
			
			// Gerar tráfego de cobertura se habilitado
			if bms.coverTraffic {
				bms.generateCoverTraffic()
			}
		}
	}
}

// processOutgoingMessages processa mensagens de saída
func (bms *BluetoothMeshService) processOutgoingMessages() {
	for {
		select {
		case <-bms.ctx.Done():
			return
		case packet := <-bms.outgoingMessages:
			// Adicionar ao cache local
			messageID := fmt.Sprintf("%x", utils.Hash(string(packet.Payload)))
			bms.addToMessageCache(messageID, packet, "self")
			
			// Enviar pacote usando o provedor de plataforma
			if err := bms.platformProvider.SendPacket(packet); err != nil {
				fmt.Printf("Erro ao enviar pacote: %v\n", err)
			}
		}
	}
}

// processIncomingMessages processa mensagens recebidas
func (bms *BluetoothMeshService) processIncomingMessages() {
	for {
		select {
		case <-bms.ctx.Done():
			return
		case packet := <-bms.incomingMessages:
			// Processar mensagem recebida
			bms.handleIncomingPacket(packet)
		}
	}
}

// scanForPeers escaneia por peers próximos
// Implementação específica da plataforma
func (bms *BluetoothMeshService) scanForPeers() {
	// Placeholder - implementação real depende da biblioteca BLE específica
	fmt.Println("Escaneando por peers...")
}

// advertise faz advertising do dispositivo
// Implementação específica da plataforma
func (bms *BluetoothMeshService) advertise() {
	// Placeholder - implementação real depende da biblioteca BLE específica
	fmt.Println("Fazendo advertising...")
}

// handleIncomingPacket processa um pacote recebido
func (bms *BluetoothMeshService) handleIncomingPacket(packet *protocol.BitchatPacket) {
	// Verificar se já vimos esta mensagem
	messageID := utils.GenerateMessageID(packet)
	if bms.seenMessages.Contains(messageID) {
		return // Ignorar mensagens duplicadas
	}
	
	// Marcar como vista
	bms.seenMessages.Add(messageID)
	
	// Verificar TTL
	if packet.TTL <= 0 {
		return // TTL expirado, não repassar
	}
	
	// Decrementar TTL para repassar
	packet.TTL--
	
	// Adicionar ao cache para store-and-forward
	senderID := string(packet.SenderID)
	bms.addToMessageCache(messageID, packet, senderID)
	
	// Verificar se é para nós
	isForUs := bms.isPacketForUs(packet)
	
	// Repassar para outros peers (relay)
	if packet.TTL > 0 {
		// Relay do pacote agora é gerenciado pelo PlatformProvider
		// Não é mais necessário chamar relayPacket
	}
	
	// Se for para nós, processar
	if isForUs {
		bms.processPacketForUs(packet)
	}
}

// isPacketForUs verifica se um pacote é destinado a este dispositivo
func (bms *BluetoothMeshService) isPacketForUs(packet *protocol.BitchatPacket) bool {
	// Broadcast é para todos
	if len(packet.RecipientID) == len(protocol.BroadcastRecipient) {
		isBroadcast := true
		for i := 0; i < len(packet.RecipientID); i++ {
			if packet.RecipientID[i] != protocol.BroadcastRecipient[i] {
				isBroadcast = false
				break
			}
		}
		if isBroadcast {
			return true
		}
	}
	
	// Verificar se é para o nosso ID
	return utils.ByteArraysEqual(packet.RecipientID, bms.deviceID)
}

// processPacketForUs processa um pacote destinado a este dispositivo
func (bms *BluetoothMeshService) processPacketForUs(packet *protocol.BitchatPacket) {
	switch packet.Type {
	case protocol.MessageTypeMessage:
		bms.handleUserMessage(packet)
	case protocol.MessageTypeAnnounce:
		bms.handleAnnounce(packet)
	case protocol.MessageTypeKeyExchange:
		bms.handleKeyExchange(packet)
	case protocol.MessageTypeDeliveryAck:
		bms.handleDeliveryAck(packet)
	case protocol.MessageTypeReadReceipt:
		bms.handleReadReceipt(packet)
	// Outros tipos de mensagem serão implementados conforme necessário
	}
}

// handleUserMessage processa uma mensagem de usuário
func (bms *BluetoothMeshService) handleUserMessage(packet *protocol.BitchatPacket) {
	senderID := string(packet.SenderID)
	
	// Verificar se temos o peer
	peer, exists := bms.getPeer(senderID)
	if !exists {
		// Não conhecemos este peer, não podemos descriptografar
		return
	}
	
	// Criar objeto de mensagem
	message := &protocol.BitchatMessage{
		ID:        utils.GenerateMessageID(packet),
		Sender:    peer.Name,
		Timestamp: packet.Timestamp,
		IsRelay:   false,
		SenderPeerID: senderID,
	}
	
	// Verificar se é privada (para nós especificamente)
	isPrivate := utils.ByteArraysEqual(packet.RecipientID, bms.deviceID)
	message.IsPrivate = isPrivate
	
	// Processar conteúdo
	if isPrivate {
		// Descriptografar mensagem privada
		decrypted, err := bms.encryptionService.Decrypt(packet.Payload, []byte(senderID), nil)
		if err == nil {
			message.Content = string(decrypted)
			message.IsEncrypted = true
		} else {
			// Falha na descriptografia
			message.Content = "[Mensagem criptografada - chave não disponível]"
			message.IsEncrypted = true
		}
	} else {
		// Mensagem broadcast
		message.Content = string(packet.Payload)
	}
	
	// Verificar assinatura se presente
	if len(packet.Signature) > 0 {
		valid, err := bms.encryptionService.Verify(packet.Signature, packet.Payload, []byte(senderID))
		if err != nil || !valid {
			// Assinatura inválida, marcar de alguma forma
			message.Content = "[AVISO: Assinatura inválida] " + message.Content
		}
	}
	
	// Enviar confirmação de entrega
	bms.sendDeliveryAck(message.ID, senderID)
	
	// Notificar delegate
	if bms.delegate != nil {
		bms.delegate.OnMessageReceived(message)
	}
}

// handleAnnounce processa um anúncio de peer
func (bms *BluetoothMeshService) handleAnnounce(packet *protocol.BitchatPacket) {
	// Extrair informações do peer do payload
	if len(packet.Payload) < 2 {
		return // Payload inválido
	}
	
	nameLen := int(packet.Payload[0])
	if len(packet.Payload) < 1+nameLen {
		return // Payload inválido
	}
	
	name := string(packet.Payload[1 : 1+nameLen])
	publicKeyData := packet.Payload[1+nameLen:]
	
	// Adicionar ou atualizar peer
	peerID := string(packet.SenderID)
	bms.addOrUpdatePeer(peerID, name, publicKeyData)
}

// handleKeyExchange processa uma troca de chaves
func (bms *BluetoothMeshService) handleKeyExchange(packet *protocol.BitchatPacket) {
	peerID := string(packet.SenderID)
	
	// Adicionar chave pública do peer
	err := bms.encryptionService.AddPeerPublicKey(peerID, packet.Payload)
	if err != nil {
		// Erro ao processar chave
		return
	}
	
	// Responder com nossa chave pública se necessário
	bms.sendKeyExchange(peerID)
}

// handleDeliveryAck processa confirmação de entrega
func (bms *BluetoothMeshService) handleDeliveryAck(packet *protocol.BitchatPacket) {
	// Implementação básica - detalhes completos dependem da estrutura de DeliveryAck
	if len(packet.Payload) < 16 { // Tamanho mínimo para um ID de mensagem
		return
	}
	
	// Extrair ID da mensagem original
	messageID := string(packet.Payload[:16])
	
	// Atualizar status de entrega
	if bms.delegate != nil {
		info := &protocol.DeliveryInfo{
			Status:    protocol.DeliveryStatusDelivered,
			Recipient: string(packet.SenderID),
			Timestamp: uint64(time.Now().UnixMilli()),
		}
		bms.delegate.OnMessageDeliveryChanged(messageID, protocol.DeliveryStatusDelivered, info)
	}
}

// handleReadReceipt processa confirmação de leitura
func (bms *BluetoothMeshService) handleReadReceipt(packet *protocol.BitchatPacket) {
	// Similar ao handleDeliveryAck, mas para status de leitura
	if len(packet.Payload) < 16 {
		return
	}
	
	messageID := string(packet.Payload[:16])
	
	if bms.delegate != nil {
		info := &protocol.DeliveryInfo{
			Status:    protocol.DeliveryStatusRead,
			Recipient: string(packet.SenderID),
			Timestamp: uint64(time.Now().UnixMilli()),
		}
		bms.delegate.OnMessageDeliveryChanged(messageID, protocol.DeliveryStatusRead, info)
	}
}

// sendDeliveryAck envia confirmação de entrega
func (bms *BluetoothMeshService) sendDeliveryAck(messageID string, recipientID string) {
	packet := &protocol.BitchatPacket{
		Version:    1,
		Type:       protocol.MessageTypeDeliveryAck,
		SenderID:   bms.deviceID,
		RecipientID: []byte(recipientID),
		Timestamp:  uint64(time.Now().UnixMilli()),
		Payload:    []byte(messageID),
		TTL:        7,
	}
	
	// Assinar
	signature, err := bms.encryptionService.Sign(packet.Payload)
	if err != nil {
		fmt.Printf("erro ao assinar pacote: %v\n", err)
		return
	}
	packet.Signature = signature
	
	// Enviar
	bms.outgoingMessages <- packet
}

// sendKeyExchange envia dados de chave pública para um peer
func (bms *BluetoothMeshService) sendKeyExchange(recipientID string) {
	// Obter dados combinados de chave pública
	publicKeyData := bms.encryptionService.GetCombinedPublicKeyData()
	
	packet := &protocol.BitchatPacket{
		Version:    1,
		Type:       protocol.MessageTypeKeyExchange,
		SenderID:   bms.deviceID,
		RecipientID: []byte(recipientID),
		Timestamp:  uint64(time.Now().UnixMilli()),
		Payload:    publicKeyData,
		TTL:        1, // TTL baixo para troca de chaves
	}
	
	// Enviar sem assinar (a própria chave pública é a prova)
	bms.outgoingMessages <- packet
}

// addToMessageCache adiciona uma mensagem ao cache
func (bms *BluetoothMeshService) addToMessageCache(messageID string, packet *protocol.BitchatPacket, originalSender string) {
	bms.messageCache.mutex.Lock()
	defer bms.messageCache.mutex.Unlock()
	
	// Verificar se já existe
	if _, exists := bms.messageCache.messages[messageID]; exists {
		return
	}
	
	// Verificar tamanho do cache
	if len(bms.messageCache.messages) >= bms.messageCache.maxSize {
		// Remover mensagem mais antiga
		var oldestID string
		var oldestTime time.Time
		first := true
		
		for id, msg := range bms.messageCache.messages {
			if first || msg.ReceivedAt.Before(oldestTime) {
				oldestID = id
				oldestTime = msg.ReceivedAt
				first = false
			}
		}
		
		if oldestID != "" {
			delete(bms.messageCache.messages, oldestID)
		}
	}
	
	// Adicionar nova mensagem
	ttl := DefaultMessageCacheTTL
	if bms.batteryMode == BatteryModeLow {
		ttl = DefaultMessageCacheTTL / 2
	} else if bms.batteryMode == BatteryModeUltraLow {
		ttl = DefaultMessageCacheTTL / 4
	}
	
	bms.messageCache.messages[messageID] = &CachedMessage{
		Packet:         packet,
		ReceivedAt:     time.Now(),
		ExpiresAt:      time.Now().Add(ttl),
		DeliveredTo:    make(map[string]bool),
		OriginalSender: originalSender,
	}
}

// Removemos broadcastToNearbyPeers e relayPacket pois agora são gerenciados pelo PlatformProvider

// cleanupExpiredMessages remove mensagens expiradas do cache
func (bms *BluetoothMeshService) cleanupExpiredMessages() {
	bms.messageCache.mutex.Lock()
	defer bms.messageCache.mutex.Unlock()
	
	now := time.Now()
	for id, msg := range bms.messageCache.messages {
		if now.After(msg.ExpiresAt) {
			delete(bms.messageCache.messages, id)
		}
	}
}

// cleanupInactivePeers remove peers inativos
func (bms *BluetoothMeshService) cleanupInactivePeers() {
	bms.mutex.Lock()
	defer bms.mutex.Unlock()
	
	threshold := time.Now().Add(-10 * time.Minute)
	for id, peer := range bms.peers {
		if peer.LastSeen.Before(threshold) {
			delete(bms.peers, id)
			
			// Notificar delegate
			if bms.delegate != nil {
				bms.delegate.OnPeerLost(id)
			}
		}
	}
}

// generateCoverTraffic gera tráfego de cobertura para privacidade
func (bms *BluetoothMeshService) generateCoverTraffic() {
	// Implementação básica - enviar pacotes vazios ou aleatórios
	if !bms.coverTraffic {
		return
	}
	
	// Gerar pacote de cover traffic apenas se estiver no modo normal de bateria
	if bms.batteryMode == BatteryModeNormal {
		packet := &protocol.BitchatPacket{
			Version:    1,
			Type:       protocol.MessageTypeAnnounce, // Usar tipo comum para não chamar atenção
			SenderID:   bms.deviceID,
			RecipientID: protocol.BroadcastRecipient,
			Timestamp:  uint64(time.Now().UnixMilli()),
			Payload:    []byte{}, // Payload vazio ou aleatório
			TTL:        1,        // TTL baixo para não sobrecarregar a rede
		}
		
		// Enviar com probabilidade baixa
		if utils.RandomInt(100) < 10 { // 10% de chance
			bms.outgoingMessages <- packet
		}
	}
}

// addOrUpdatePeer adiciona ou atualiza informações de um peer
func (bms *BluetoothMeshService) addOrUpdatePeer(peerID string, name string, publicKeyData []byte) {
	bms.mutex.Lock()
	defer bms.mutex.Unlock()
	
	isNew := false
	peer, exists := bms.peers[peerID]
	if !exists {
		peer = &Peer{
			ID:   peerID,
			Name: name,
		}
		bms.peers[peerID] = peer
		isNew = true
	}
	
	// Atualizar informações
	peer.LastSeen = time.Now()
	peer.Name = name
	if publicKeyData != nil {
		peer.PublicKeyData = publicKeyData
		
		// Adicionar chave pública ao serviço de criptografia
		bms.encryptionService.AddPeerPublicKey(peerID, publicKeyData)
	}
	
	// Notificar delegate se for um novo peer
	if isNew && bms.delegate != nil {
		bms.delegate.OnPeerDiscovered(peerID, name)
	}
}

// getPeer obtém informações de um peer
func (bms *BluetoothMeshService) getPeer(peerID string) (*Peer, bool) {
	bms.mutex.RLock()
	defer bms.mutex.RUnlock()
	
	peer, exists := bms.peers[peerID]
	return peer, exists
}

// findPeerIDByNickname busca um peer pelo nickname
func (bms *BluetoothMeshService) findPeerIDByNickname(nickname string) string {
	bms.mutex.RLock()
	defer bms.mutex.RUnlock()
	
	for id, peer := range bms.peers {
		if peer.Name == nickname {
			return id
		}
	}
	
	return ""
}
