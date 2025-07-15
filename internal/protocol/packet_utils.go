package protocol

// GeneratePacketID gera um ID para o pacote usando a função existente
func GeneratePacketID(packet *BitchatPacket) string {
	return generatePacketID(packet)
}
