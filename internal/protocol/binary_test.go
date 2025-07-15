package protocol

import (
	"bytes"
	"testing"
	"time"
)

func TestBitchatPacket(t *testing.T) {
	t.Run("Codificação e decodificação de pacote", func(t *testing.T) {
		// Criar pacote de teste
		original := &BitchatPacket{
			Version:     CurrentProtocolVersion,
			ID:          "test-packet-id",
			Type:        MessageTypePrivate,
			SenderID:    "sender-123",
			RecipientID: "recipient-456",
			ChannelID:   "",
			Timestamp:   uint64(time.Now().UnixMilli()),
			TTL:         5,
			Payload:     []byte("Conteúdo da mensagem de teste"),
			Signature:   []byte("assinatura-simulada"),
		}

		// Codificar pacote
		encoded, err := original.Encode()
		if err != nil {
			t.Fatalf("Erro ao codificar pacote: %v", err)
		}

		// Decodificar pacote
		decoded, err := DecodePacket(encoded)
		if err != nil {
			t.Fatalf("Erro ao decodificar pacote: %v", err)
		}

		// Verificar se os campos foram preservados
		if decoded.Version != original.Version {
			t.Errorf("Versão não corresponde: esperado %d, obtido %d", original.Version, decoded.Version)
		}
		if decoded.ID != original.ID {
			t.Errorf("ID não corresponde: esperado %s, obtido %s", original.ID, decoded.ID)
		}
		if decoded.Type != original.Type {
			t.Errorf("Tipo não corresponde: esperado %d, obtido %d", original.Type, decoded.Type)
		}
		if decoded.SenderID != original.SenderID {
			t.Errorf("SenderID não corresponde: esperado %s, obtido %s", original.SenderID, decoded.SenderID)
		}
		if decoded.RecipientID != original.RecipientID {
			t.Errorf("RecipientID não corresponde: esperado %s, obtido %s", original.RecipientID, decoded.RecipientID)
		}
		if decoded.ChannelID != original.ChannelID {
			t.Errorf("ChannelID não corresponde: esperado %s, obtido %s", original.ChannelID, decoded.ChannelID)
		}
		if decoded.Timestamp != original.Timestamp {
			t.Errorf("Timestamp não corresponde: esperado %d, obtido %d", original.Timestamp, decoded.Timestamp)
		}
		if decoded.TTL != original.TTL {
			t.Errorf("TTL não corresponde: esperado %d, obtido %d", original.TTL, decoded.TTL)
		}
		if !bytes.Equal(decoded.Payload, original.Payload) {
			t.Errorf("Payload não corresponde: esperado %v, obtido %v", original.Payload, decoded.Payload)
		}
		if !bytes.Equal(decoded.Signature, original.Signature) {
			t.Errorf("Signature não corresponde: esperado %v, obtido %v", original.Signature, decoded.Signature)
		}
	})

	t.Run("Codificação e decodificação de mensagem de canal", func(t *testing.T) {
		// Criar pacote de canal
		original := &BitchatPacket{
			Version:   CurrentProtocolVersion,
			ID:        "channel-packet-id",
			Type:      MessageTypeChannel,
			SenderID:  "sender-123",
			ChannelID: "channel-general",
			Timestamp: uint64(time.Now().UnixMilli()),
			TTL:       5,
			Payload:   []byte("Mensagem para o canal geral"),
			Signature: []byte("assinatura-canal"),
		}

		// Codificar pacote
		encoded, err := original.Encode()
		if err != nil {
			t.Fatalf("Erro ao codificar pacote de canal: %v", err)
		}

		// Decodificar pacote
		decoded, err := DecodePacket(encoded)
		if err != nil {
			t.Fatalf("Erro ao decodificar pacote de canal: %v", err)
		}

		// Verificar campos específicos de canal
		if decoded.Type != MessageTypeChannel {
			t.Errorf("Tipo não corresponde: esperado %d, obtido %d", MessageTypeChannel, decoded.Type)
		}
		if decoded.ChannelID != original.ChannelID {
			t.Errorf("ChannelID não corresponde: esperado %s, obtido %s", original.ChannelID, decoded.ChannelID)
		}
		if decoded.RecipientID != "" {
			t.Errorf("RecipientID deveria ser vazio para mensagem de canal, obtido %s", decoded.RecipientID)
		}
	})

	t.Run("Validação de pacote", func(t *testing.T) {
		// Pacote válido
		validPacket := &BitchatPacket{
			Version:     CurrentProtocolVersion,
			ID:          "valid-packet",
			Type:        MessageTypePrivate,
			SenderID:    "sender-123",
			RecipientID: "recipient-456",
			Timestamp:   uint64(time.Now().UnixMilli()),
			TTL:         5,
			Payload:     []byte("Conteúdo válido"),
			Signature:   []byte("assinatura"),
		}

		if err := validPacket.Validate(); err != nil {
			t.Errorf("Pacote válido falhou na validação: %v", err)
		}

		// Pacote sem ID
		invalidPacket1 := &BitchatPacket{
			Version:     CurrentProtocolVersion,
			ID:          "",
			Type:        MessageTypePrivate,
			SenderID:    "sender-123",
			RecipientID: "recipient-456",
			Timestamp:   uint64(time.Now().UnixMilli()),
			Payload:     []byte("Conteúdo"),
		}

		if err := invalidPacket1.Validate(); err == nil {
			t.Error("Pacote sem ID deveria falhar na validação")
		}

		// Pacote sem SenderID
		invalidPacket2 := &BitchatPacket{
			Version:     CurrentProtocolVersion,
			ID:          "packet-id",
			Type:        MessageTypePrivate,
			SenderID:    "",
			RecipientID: "recipient-456",
			Timestamp:   uint64(time.Now().UnixMilli()),
			Payload:     []byte("Conteúdo"),
		}

		if err := invalidPacket2.Validate(); err == nil {
			t.Error("Pacote sem SenderID deveria falhar na validação")
		}

		// Pacote privado sem RecipientID
		invalidPacket3 := &BitchatPacket{
			Version:   CurrentProtocolVersion,
			ID:        "packet-id",
			Type:      MessageTypePrivate,
			SenderID:  "sender-123",
			Timestamp: uint64(time.Now().UnixMilli()),
			Payload:   []byte("Conteúdo"),
		}

		if err := invalidPacket3.Validate(); err == nil {
			t.Error("Pacote privado sem RecipientID deveria falhar na validação")
		}

		// Pacote de canal sem ChannelID
		invalidPacket4 := &BitchatPacket{
			Version:   CurrentProtocolVersion,
			ID:        "packet-id",
			Type:      MessageTypeChannel,
			SenderID:  "sender-123",
			Timestamp: uint64(time.Now().UnixMilli()),
			Payload:   []byte("Conteúdo"),
		}

		if err := invalidPacket4.Validate(); err == nil {
			t.Error("Pacote de canal sem ChannelID deveria falhar na validação")
		}

		// Pacote com versão incompatível
		invalidPacket5 := &BitchatPacket{
			Version:     CurrentProtocolVersion + 10,
			ID:          "packet-id",
			Type:        MessageTypePrivate,
			SenderID:    "sender-123",
			RecipientID: "recipient-456",
			Timestamp:   uint64(time.Now().UnixMilli()),
			Payload:     []byte("Conteúdo"),
		}

		if err := invalidPacket5.Validate(); err == nil {
			t.Error("Pacote com versão incompatível deveria falhar na validação")
		}
	})

	t.Run("Fragmentação e reconstrução", func(t *testing.T) {
		// Criar pacote grande
		largePayload := make([]byte, MaxPayloadSize*3) // 3x o tamanho máximo
		for i := range largePayload {
			largePayload[i] = byte(i % 256)
		}

		original := &BitchatPacket{
			Version:     CurrentProtocolVersion,
			ID:          "large-packet",
			Type:        MessageTypePrivate,
			SenderID:    "sender-123",
			RecipientID: "recipient-456",
			Timestamp:   uint64(time.Now().UnixMilli()),
			TTL:         5,
			Payload:     largePayload,
			Signature:   []byte("assinatura-grande"),
		}

		// Fragmentar pacote
		fragments, err := original.Fragment()
		if err != nil {
			t.Fatalf("Erro ao fragmentar pacote: %v", err)
		}

		// Verificar número de fragmentos
		expectedFragments := (len(largePayload) + MaxPayloadSize - 1) / MaxPayloadSize
		if len(fragments) != expectedFragments {
			t.Errorf("Número de fragmentos esperado: %d, obtido: %d", expectedFragments, len(fragments))
		}

		// Verificar se cada fragmento tem o mesmo ID base
		for i, fragment := range fragments {
			if !bytes.HasPrefix([]byte(fragment.ID), []byte(original.ID)) {
				t.Errorf("Fragmento %d não tem o ID base correto", i)
			}
		}

		// Reconstruir pacote a partir dos fragmentos
		fragmentMap := make(map[string]*BitchatPacket)
		for _, fragment := range fragments {
			fragmentMap[fragment.ID] = fragment
		}

		reconstructed, complete := ReconstructPacket(fragments[0], fragmentMap)
		if !complete {
			t.Error("Reconstrução do pacote não foi completada")
		}
		if reconstructed == nil {
			t.Fatal("Pacote reconstruído é nil")
		}

		// Verificar se o pacote reconstruído é igual ao original
		if reconstructed.ID != original.ID {
			t.Errorf("ID não corresponde após reconstrução: esperado %s, obtido %s", original.ID, reconstructed.ID)
		}
		if !bytes.Equal(reconstructed.Payload, original.Payload) {
			t.Error("Payload não corresponde após reconstrução")
		}
		if reconstructed.Type != original.Type {
			t.Errorf("Tipo não corresponde após reconstrução: esperado %d, obtido %d", original.Type, reconstructed.Type)
		}
	})

	t.Run("Reconstrução parcial", func(t *testing.T) {
		// Criar pacote grande
		largePayload := make([]byte, MaxPayloadSize*2) // 2x o tamanho máximo
		for i := range largePayload {
			largePayload[i] = byte(i % 256)
		}

		original := &BitchatPacket{
			Version:     CurrentProtocolVersion,
			ID:          "partial-packet",
			Type:        MessageTypePrivate,
			SenderID:    "sender-123",
			RecipientID: "recipient-456",
			Timestamp:   uint64(time.Now().UnixMilli()),
			TTL:         5,
			Payload:     largePayload,
		}

		// Fragmentar pacote
		fragments, err := original.Fragment()
		if err != nil {
			t.Fatalf("Erro ao fragmentar pacote: %v", err)
		}

		// Remover um fragmento para simular perda
		fragmentMap := make(map[string]*BitchatPacket)
		for i, fragment := range fragments {
			if i != 1 { // Pular o segundo fragmento
				fragmentMap[fragment.ID] = fragment
			}
		}

		// Tentar reconstruir com fragmentos faltando
		reconstructed, complete := ReconstructPacket(fragments[0], fragmentMap)
		if complete {
			t.Error("Reconstrução não deveria estar completa com fragmentos faltando")
		}
		if reconstructed != nil {
			t.Error("Pacote reconstruído deveria ser nil quando incompleto")
		}
	})

	t.Run("Conversão para Message", func(t *testing.T) {
		// Pacote privado
		privatePacket := &BitchatPacket{
			ID:          "private-msg",
			Type:        MessageTypePrivate,
			SenderID:    "sender-123",
			RecipientID: "recipient-456",
			Timestamp:   uint64(time.Now().UnixMilli()),
			Payload:     []byte("Mensagem privada"),
		}

		privateMsg := privatePacket.ToMessage()
		if privateMsg.ID != privatePacket.ID {
			t.Errorf("ID não corresponde: esperado %s, obtido %s", privatePacket.ID, privateMsg.ID)
		}
		if privateMsg.SenderID != privatePacket.SenderID {
			t.Errorf("SenderID não corresponde: esperado %s, obtido %s", privatePacket.SenderID, privateMsg.SenderID)
		}
		if privateMsg.RecipientID != privatePacket.RecipientID {
			t.Errorf("RecipientID não corresponde: esperado %s, obtido %s", privatePacket.RecipientID, privateMsg.RecipientID)
		}
		if privateMsg.ChannelID != "" {
			t.Errorf("ChannelID deveria ser vazio para mensagem privada, obtido %s", privateMsg.ChannelID)
		}
		if !bytes.Equal(privateMsg.Content, privatePacket.Payload) {
			t.Error("Content não corresponde ao Payload")
		}

		// Pacote de canal
		channelPacket := &BitchatPacket{
			ID:        "channel-msg",
			Type:      MessageTypeChannel,
			SenderID:  "sender-123",
			ChannelID: "channel-general",
			Timestamp: uint64(time.Now().UnixMilli()),
			Payload:   []byte("Mensagem de canal"),
		}

		channelMsg := channelPacket.ToMessage()
		if channelMsg.ID != channelPacket.ID {
			t.Errorf("ID não corresponde: esperado %s, obtido %s", channelPacket.ID, channelMsg.ID)
		}
		if channelMsg.ChannelID != channelPacket.ChannelID {
			t.Errorf("ChannelID não corresponde: esperado %s, obtido %s", channelPacket.ChannelID, channelMsg.ChannelID)
		}
		if channelMsg.RecipientID != "" {
			t.Errorf("RecipientID deveria ser vazio para mensagem de canal, obtido %s", channelMsg.RecipientID)
		}
	})
}
