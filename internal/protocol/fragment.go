package protocol

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"time"
)

// FragmentMeta contém metadados sobre fragmentos de um pacote
type FragmentMeta struct {
	PacketID      string    // ID do pacote original
	TotalFragments int       // Número total de fragmentos
	ReceivedFragments int    // Número de fragmentos recebidos
	Timestamp     time.Time  // Timestamp de quando o primeiro fragmento foi recebido
	SenderID      []byte     // ID do remetente
}

// FragmentData representa um fragmento de dados
type FragmentData struct {
	PacketID      string    // ID do pacote original
	FragmentIndex int       // Índice do fragmento (0-based)
	TotalFragments int      // Número total de fragmentos
	Data          []byte    // Dados do fragmento
	IsLast        bool      // Indica se é o último fragmento
}

// EncodeFragment codifica um fragmento em bytes para transmissão
func EncodeFragment(packetID string, fragmentIndex int, totalFragments int) ([]byte, error) {
	fragment := &FragmentData{
		PacketID:      packetID,
		FragmentIndex: fragmentIndex,
		TotalFragments: totalFragments,
		IsLast:        fragmentIndex == totalFragments-1,
		Data:          []byte{},
	}
	// Formato do fragmento:
	// [1 byte: versão] [16 bytes: PacketID] [1 byte: FragmentIndex] [1 byte: TotalFragments] [1 byte: IsLast] [N bytes: Data]
	
	// Verificar tamanho do PacketID
	if len(fragment.PacketID) > 32 {
		return nil, fmt.Errorf("PacketID muito longo (máximo 32 caracteres)")
	}
	
	// Calcular tamanho total
	totalSize := 1 + 32 + 1 + 1 + 1 + len(fragment.Data)
	result := make([]byte, totalSize)
	
	// Versão do protocolo
	result[0] = 1
	
	// PacketID (padded com zeros se necessário)
	copy(result[1:33], []byte(fragment.PacketID))
	
	// FragmentIndex
	result[33] = byte(fragment.FragmentIndex)
	
	// TotalFragments
	result[34] = byte(fragment.TotalFragments)
	
	// IsLast
	if fragment.IsLast {
		result[35] = 1
	} else {
		result[35] = 0
	}
	
	// Dados
	copy(result[36:], fragment.Data)
	
	return result, nil
}

// DecodeFragment decodifica bytes em um FragmentData
func DecodeFragment(data []byte) (string, int, int, []byte, error) {
	// Verificar tamanho mínimo
	if len(data) < 36 {
		return "", 0, 0, nil, fmt.Errorf("dados muito curtos para um fragmento válido")
	}
	
	// Verificar versão
	if data[0] != 1 {
		return "", 0, 0, nil, fmt.Errorf("versão de fragmento não suportada: %d", data[0])
	}
	
	// Extrair PacketID
	packetIDBytes := data[1:33]
	packetID := string(packetIDBytes)
	
	// Remover padding de zeros
	for i := len(packetID) - 1; i >= 0; i-- {
		if packetID[i] != 0 {
			packetID = packetID[:i+1]
			break
		}
	}
	
	// Extrair FragmentIndex
	fragmentIndex := int(data[33])
	
	// Extrair TotalFragments
	totalFragments := int(data[34])
	
	// Extrair isLast (não utilizado na nova implementação, mas mantido para compatibilidade futura)
	_ = data[35] == 1
	
	// Extrair dados
	fragmentData := make([]byte, len(data)-36)
	copy(fragmentData, data[36:])
	
	return packetID, fragmentIndex, totalFragments, fragmentData, nil
}

// EncodePacket codifica um BitchatPacket em bytes para transmissão
func EncodePacket(packet *BitchatPacket) ([]byte, error) {
	// Calcular tamanho total
	totalSize := 1 + // Versão
		1 + // Tipo
		8 + // SenderID (assumindo tamanho fixo)
		8 + // RecipientID (assumindo tamanho fixo)
		8 + // Timestamp
		1 + // TTL
		4 + // Tamanho do payload
		len(packet.Payload) +
		len(packet.Signature)
	
	result := make([]byte, totalSize)
	offset := 0
	
	// Versão
	result[offset] = packet.Version
	offset++
	
	// Tipo
	result[offset] = byte(packet.Type)
	offset++
	
	// SenderID
	copy(result[offset:offset+8], packet.SenderID)
	offset += 8
	
	// RecipientID
	copy(result[offset:offset+8], packet.RecipientID)
	offset += 8
	
	// Timestamp
	binary.BigEndian.PutUint64(result[offset:offset+8], packet.Timestamp)
	offset += 8
	
	// TTL
	result[offset] = packet.TTL
	offset++
	
	// Tamanho do payload
	binary.BigEndian.PutUint32(result[offset:offset+4], uint32(len(packet.Payload)))
	offset += 4
	
	// Payload
	copy(result[offset:offset+len(packet.Payload)], packet.Payload)
	offset += len(packet.Payload)
	
	// Signature (se presente)
	if len(packet.Signature) > 0 {
		copy(result[offset:], packet.Signature)
	}
	
	return result, nil
}

// DecodePacket decodifica bytes em um BitchatPacket
func DecodePacket(data []byte) (*BitchatPacket, error) {
	// Verificar tamanho mínimo
	if len(data) < 31 { // 1+1+8+8+8+1+4
		return nil, fmt.Errorf("dados muito curtos para um pacote válido")
	}
	
	offset := 0
	
	// Versão
	version := data[offset]
	offset++
	
	// Tipo
	msgType := MessageType(data[offset])
	offset++
	
	// SenderID
	senderID := make([]byte, 8)
	copy(senderID, data[offset:offset+8])
	offset += 8
	
	// RecipientID
	recipientID := make([]byte, 8)
	copy(recipientID, data[offset:offset+8])
	offset += 8
	
	// Timestamp
	timestamp := binary.BigEndian.Uint64(data[offset:offset+8])
	offset += 8
	
	// TTL
	ttl := data[offset]
	offset++
	
	// Tamanho do payload
	payloadSize := binary.BigEndian.Uint32(data[offset:offset+4])
	offset += 4
	
	// Verificar se há dados suficientes para o payload
	if uint32(len(data)-offset) < payloadSize {
		return nil, fmt.Errorf("dados insuficientes para o payload declarado")
	}
	
	// Payload
	payload := make([]byte, payloadSize)
	copy(payload, data[offset:offset+int(payloadSize)])
	offset += int(payloadSize)
	
	// Signature (resto dos dados)
	signature := make([]byte, len(data)-offset)
	if len(signature) > 0 {
		copy(signature, data[offset:])
	}
	
	// Criar pacote
	packet := &BitchatPacket{
		Version:    version,
		Type:       msgType,
		SenderID:   senderID,
		RecipientID: recipientID,
		Timestamp:  timestamp,
		Payload:    payload,
		Signature:  signature,
		TTL:        ttl,
	}
	
	// Gerar ID
	packet.ID = generatePacketID(packet)
	
	return packet, nil
}

// ExtractMetadataFromServiceData extrai metadados de fragmentação dos dados de serviço BLE
func ExtractMetadataFromServiceData(serviceData []byte) *FragmentMeta {
	// Verificar tamanho mínimo
	if len(serviceData) < 5 {
		return &FragmentMeta{
			PacketID:      "error",
			TotalFragments: 0,
			ReceivedFragments: 0,
			Timestamp:     time.Now(),
		}
	}
	
	// Verificar se é um pacote Bitchat (deve começar com "BTCHT")
	if len(serviceData) < 5 || string(serviceData[:5]) != "BTCHT" {
		return &FragmentMeta{
			PacketID:      "error",
			TotalFragments: 0,
			ReceivedFragments: 0,
			Timestamp:     time.Now(),
		}
	}
	
	// Extrair metadados
	// Formato: "BTCHT" + [1 byte: tipo] + [32 bytes: packetID] + [1 byte: totalFragments]
	if len(serviceData) < 39 {
		return &FragmentMeta{
			PacketID:      "error",
			TotalFragments: 0,
			ReceivedFragments: 0,
			Timestamp:     time.Now(),
		}
	}
	
	// Tipo de pacote
	packetType := MessageType(serviceData[5])
	
	// Verificar se é um fragmento
	if packetType != MessageTypeFragmentStart && 
	   packetType != MessageTypeFragmentContinue && 
	   packetType != MessageTypeFragmentEnd {
		return &FragmentMeta{
			PacketID:      "error",
			TotalFragments: 0,
			ReceivedFragments: 0,
			Timestamp:     time.Now(),
		}
	}
	
	// PacketID
	packetIDBytes := serviceData[6:38]
	packetID := string(packetIDBytes)
	
	// Remover padding de zeros
	for i := len(packetID) - 1; i >= 0; i-- {
		if packetID[i] != 0 {
			packetID = packetID[:i+1]
			break
		}
	}
	
	// Total de fragmentos
	totalFragments := int(serviceData[38])
	
	return &FragmentMeta{
		PacketID:      packetID,
		TotalFragments: totalFragments,
		ReceivedFragments: 0,
		Timestamp:     time.Now(),
	}
}

// ExtractMetadataFromManufacturerData extrai metadados de fragmentação dos dados do fabricante BLE
func ExtractMetadataFromManufacturerData(manufacturerData []byte) *FragmentMeta {
	// Implementação simplificada para compilação
	return &FragmentMeta{
		PacketID:      "temp-id",
		TotalFragments: 1,
		ReceivedFragments: 0,
		Timestamp:     time.Now(),
	}
}

// IsFragment verifica se um pacote é um fragmento
func IsFragment(msgType MessageType) bool {
	return msgType == MessageTypeFragmentStart ||
		msgType == MessageTypeFragmentContinue ||
		msgType == MessageTypeFragmentEnd
}

// ReassembleFragments reconstrói um pacote a partir de fragmentos
func ReassembleFragments(fragments map[int][]byte, totalFragments int) ([]byte, error) {
	// Verificar se todos os fragmentos estão presentes
	if len(fragments) != totalFragments {
		return nil, fmt.Errorf("fragmentos incompletos: %d/%d", len(fragments), totalFragments)
	}
	
	// Calcular tamanho total
	totalSize := 0
	for _, data := range fragments {
		totalSize += len(data)
	}
	
	// Concatenar fragmentos na ordem correta
	result := make([]byte, 0, totalSize)
	for i := 0; i < totalFragments; i++ {
		data, ok := fragments[i]
		if !ok {
			return nil, fmt.Errorf("fragmento %d ausente", i)
		}
		result = append(result, data...)
	}
	
	return result, nil
}

// GenerateCoverTrafficPacket gera um pacote de tráfego de cobertura
func GenerateCoverTrafficPacket() *BitchatPacket {
	// Gerar ID aleatório para o remetente
	senderID := make([]byte, 8)
	_, _ = rand.Read(senderID)
	
	// Gerar payload aleatório
	payloadSize := 16 + rand.Intn(32) // Entre 16 e 48 bytes
	payload := make([]byte, payloadSize)
	_, _ = rand.Read(payload)
	
	// Criar pacote
	return NewBroadcastPacket(MessageTypeAnnounce, senderID, payload)
}
