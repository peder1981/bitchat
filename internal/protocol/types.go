package protocol

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"time"
)

// MessageType define os tipos de mensagens no protocolo
type MessageType uint8

const (
	MessageTypeAnnounce          MessageType = 0x01
	MessageTypeKeyExchange       MessageType = 0x02
	MessageTypeLeave             MessageType = 0x03
	MessageTypeMessage           MessageType = 0x04 // Todas as mensagens de usuário (privadas e broadcast)
	MessageTypeFragmentStart     MessageType = 0x05
	MessageTypeFragmentContinue  MessageType = 0x06
	MessageTypeFragmentEnd       MessageType = 0x07
	MessageTypeChannelAnnounce   MessageType = 0x08 // Anunciar status de canal protegido por senha
	MessageTypeChannelRetention  MessageType = 0x09 // Anunciar status de retenção de canal
	MessageTypeDeliveryAck       MessageType = 0x0A // Confirmar recebimento de mensagem
	MessageTypeDeliveryStatusReq MessageType = 0x0B // Solicitar atualização de status de entrega
	MessageTypeReadReceipt       MessageType = 0x0C // Mensagem foi lida/visualizada
	MessageTypeText             MessageType = 0x0D // Mensagem de texto simples para testes
)

// SpecialRecipients define IDs de destinatários especiais
var BroadcastRecipient = []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF} // Todos 0xFF = broadcast

// BitchatPacket é a estrutura principal de pacotes do protocolo
type BitchatPacket struct {
	Version    uint8
	Type       MessageType
	SenderID   []byte
	RecipientID []byte
	Timestamp  uint64
	Payload    []byte
	Signature  []byte
	TTL        uint8
	ID         string // ID único do pacote para deduplicação e tracking
	Nonce      []byte // Nonce para criptografia (compatível com testes)
}

// NewBitchatPacket cria um novo pacote com valores padrão
func NewBitchatPacket(msgType MessageType, senderID []byte, recipientID []byte, payload []byte) *BitchatPacket {
	packet := &BitchatPacket{
		Version:    1,
		Type:       msgType,
		SenderID:   senderID,
		RecipientID: recipientID,
		Timestamp:  uint64(time.Now().UnixMilli()),
		Payload:    payload,
		Signature:  nil,
		TTL:        7, // Valor padrão para TTL
	}
	
	// Gerar ID único para o pacote
	packet.ID = generatePacketID(packet)
	
	return packet
}

// NewBroadcastPacket cria um novo pacote de broadcast
func NewBroadcastPacket(msgType MessageType, senderID []byte, payload []byte) *BitchatPacket {
	return NewBitchatPacket(msgType, senderID, BroadcastRecipient, payload)
}

// BitchatMessage representa uma mensagem de chat
type BitchatMessage struct {
	ID               string
	Sender           string
	Content          string
	Timestamp        uint64     // Timestamp em milissegundos desde epoch
	IsRelay          bool
	OriginalSender   string
	IsPrivate        bool
	RecipientNickname string
	SenderPeerID     string
	Mentions         []string
	Channel          string
	EncryptedContent []byte
	IsEncrypted      bool
	DeliveryStatus   DeliveryStatus
}

// DeliveryStatus representa o status de entrega de uma mensagem
type DeliveryStatus int

const (
	DeliveryStatusSending DeliveryStatus = iota
	DeliveryStatusSent
	DeliveryStatusDelivered
	DeliveryStatusRead
	DeliveryStatusFailed
	DeliveryStatusPartiallyDelivered
)

// DeliveryInfo armazena informações detalhadas sobre entrega
type DeliveryInfo struct {
	Status      DeliveryStatus
	Recipient   string
	Timestamp   uint64      // Timestamp em milissegundos desde epoch
	FailReason  string
	ReachedPeers int
	TotalPeers   int
	Attempts    int        // Número de tentativas de entrega
	Error       string     // Mensagem de erro detalhada, se houver
}

// DeliveryAck representa uma confirmação de entrega
type DeliveryAck struct {
	OriginalMessageID string
	AckID             string
	RecipientID       string
	RecipientNickname string
	Timestamp         time.Time
	HopCount          uint8
}

// ReadReceipt representa uma confirmação de leitura
type ReadReceipt struct {
	OriginalMessageID string
	ReceiptID         string
	ReaderID          string
	ReaderNickname    string
	Timestamp         time.Time
}

// generatePacketID gera um ID único para um pacote
// Combina timestamp, tipo de mensagem, sender e recipient para garantir unicidade
func generatePacketID(packet *BitchatPacket) string {
	// Criar um hash com os campos principais do pacote
	h := sha256.New()
	
	// Adicionar timestamp para unicidade
	binary.Write(h, binary.BigEndian, packet.Timestamp)
	
	// Adicionar tipo de mensagem
	h.Write([]byte{byte(packet.Type)})
	
	// Adicionar sender e recipient
	h.Write(packet.SenderID)
	h.Write(packet.RecipientID)
	
	// Adicionar um hash dos primeiros bytes do payload (se existir)
	if len(packet.Payload) > 0 {
		payloadLen := len(packet.Payload)
		if payloadLen > 16 {
			payloadLen = 16
		}
		h.Write(packet.Payload[:payloadLen])
	}
	
	// Gerar bytes aleatórios para garantir unicidade mesmo com campos idênticos
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	h.Write(randomBytes)
	
	// Retornar os primeiros 16 bytes como string hex
	return hex.EncodeToString(h.Sum(nil)[:16])
}
