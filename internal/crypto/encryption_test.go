package crypto

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/nacl/box"
)

func TestEncryptionService(t *testing.T) {
	// Criar diretório temporário para testes
	testDir, err := os.MkdirTemp("", "bitchat-crypto-test")
	if err != nil {
		t.Fatalf("Erro ao criar diretório temporário: %v", err)
	}
	defer os.RemoveAll(testDir)

	t.Run("Criação do serviço", func(t *testing.T) {
		config := &EncryptionConfig{
			KeysDir: testDir,
		}

		service, err := NewEncryptionService(config)
		if err != nil {
			t.Fatalf("Erro ao criar EncryptionService: %v", err)
		}
		if service == nil {
			t.Fatal("NewEncryptionService retornou nil")
		}

		// Verificar se as chaves foram geradas
		if service.GetPeerID() == "" {
			t.Error("PeerID não foi gerado")
		}

		// Verificar se os arquivos de chaves foram criados
		keyFiles := []string{
			filepath.Join(testDir, "identity.key"),
			filepath.Join(testDir, "signing.key"),
		}

		for _, file := range keyFiles {
			if _, err := os.Stat(file); os.IsNotExist(err) {
				t.Errorf("Arquivo de chave não foi criado: %s", file)
			}
		}
	})

	t.Run("Persistência de chaves", func(t *testing.T) {
		keyDir := filepath.Join(testDir, "persistence")
		if err := os.MkdirAll(keyDir, 0755); err != nil {
			t.Fatalf("Erro ao criar diretório para teste de persistência: %v", err)
		}

		// Criar primeiro serviço
		config := &EncryptionConfig{
			KeysDir: keyDir,
		}

		service1, err := NewEncryptionService(config)
		if err != nil {
			t.Fatalf("Erro ao criar primeiro serviço: %v", err)
		}

		peerID1 := service1.GetPeerID()
		publicKey1 := service1.GetPublicKey()

		// Criar segundo serviço (deve carregar as mesmas chaves)
		service2, err := NewEncryptionService(config)
		if err != nil {
			t.Fatalf("Erro ao criar segundo serviço: %v", err)
		}

		peerID2 := service2.GetPeerID()
		publicKey2 := service2.GetPublicKey()

		// Verificar se as identidades são iguais
		if peerID1 != peerID2 {
			t.Errorf("PeerIDs não correspondem: %s != %s", peerID1, peerID2)
		}

		// Verificar se as chaves públicas são iguais
		if !bytes.Equal(publicKey1, publicKey2) {
			t.Error("Chaves públicas não correspondem")
		}
	})

	t.Run("Criptografia e descriptografia", func(t *testing.T) {
		config := &EncryptionConfig{
			KeysDir: filepath.Join(testDir, "crypto"),
		}

		service, err := NewEncryptionService(config)
		if err != nil {
			t.Fatalf("Erro ao criar serviço: %v", err)
		}

		// Gerar par de chaves para peer remoto simulado
		publicKey, privateKey, err := box.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("Erro ao gerar chaves para peer remoto: %v", err)
		}

		// Dados para criptografar
		plaintext := []byte("Mensagem secreta para teste de criptografia")

		// Criptografar dados
		encrypted, nonce, err := service.Encrypt(plaintext, publicKey[:])
		if err != nil {
			t.Fatalf("Erro ao criptografar dados: %v", err)
		}

		// Descriptografar dados (simulando o peer remoto)
		var servicePublicKey [32]byte
		copy(servicePublicKey[:], service.GetPublicKey())

		// Converter nonce para o formato esperado pelo NaCl
		var nonceArray [24]byte
		copy(nonceArray[:], nonce)

		decrypted, ok := box.Open(nil, encrypted, &nonceArray, &servicePublicKey, privateKey)
		if !ok {
			t.Fatal("Falha ao descriptografar dados")
		}

		// Verificar se os dados descriptografados correspondem ao original
		if !bytes.Equal(plaintext, decrypted) {
			t.Error("Dados descriptografados não correspondem ao original")
		}

		// Testar descriptografia pelo serviço
		// Reutilizar a mesma nonceArray já definida acima
		remoteEncrypted := box.Seal(nil, plaintext, &nonceArray, &servicePublicKey, privateKey)

		serviceDecrypted, err := service.Decrypt(remoteEncrypted, publicKey[:], nonce)
		if err != nil {
			t.Fatalf("Erro ao descriptografar no serviço: %v", err)
		}

		if !bytes.Equal(plaintext, serviceDecrypted) {
			t.Error("Dados descriptografados pelo serviço não correspondem ao original")
		}
	})

	t.Run("Assinatura e verificação", func(t *testing.T) {
		config := &EncryptionConfig{
			KeysDir: filepath.Join(testDir, "signing"),
		}

		service, err := NewEncryptionService(config)
		if err != nil {
			t.Fatalf("Erro ao criar serviço: %v", err)
		}

		// Dados para assinar
		data := []byte("Dados para assinar e verificar")

		// Assinar dados
		signature, err := service.Sign(data)
		if err != nil {
			t.Fatalf("Erro ao assinar dados: %v", err)
		}

		// Verificar assinatura
		valid, err := service.Verify(data, signature, service.GetSigningPublicKey())
		if err != nil {
			t.Fatalf("Erro ao verificar assinatura: %v", err)
		}
		if !valid {
			t.Error("Assinatura válida foi rejeitada")
		}

		// Verificar rejeição de assinatura inválida
		invalidSignature := make([]byte, ed25519.SignatureSize)
		copy(invalidSignature, signature)
		invalidSignature[0] ^= 0x01 // Alterar um bit

		valid, err = service.Verify(data, invalidSignature, service.GetSigningPublicKey())
		if err != nil {
			t.Fatalf("Erro ao verificar assinatura inválida: %v", err)
		}
		if valid {
			t.Error("Assinatura inválida foi aceita")
		}
	})

	t.Run("Derivação de chave de canal", func(t *testing.T) {
		config := &EncryptionConfig{
			KeysDir: filepath.Join(testDir, "channel"),
		}

		service, err := NewEncryptionService(config)
		if err != nil {
			t.Fatalf("Erro ao criar serviço: %v", err)
		}

		// Derivar chave de canal
		channelName := "canal-teste"
		password := "senha-secreta"

		key1, salt, err := service.DeriveChannelKey(channelName, password, nil)
		if err != nil {
			t.Fatalf("Erro ao derivar chave de canal: %v", err)
		}

		// Derivar novamente com o mesmo salt
		key2, _, err := service.DeriveChannelKey(channelName, password, salt)
		if err != nil {
			t.Fatalf("Erro ao derivar chave de canal novamente: %v", err)
		}

		// Verificar se as chaves são iguais
		if !bytes.Equal(key1, key2) {
			t.Error("Chaves derivadas não correspondem")
		}

		// Verificar rejeição de senha incorreta
		key3, _, err := service.DeriveChannelKey(channelName, "senha-errada", salt)
		if err != nil {
			t.Fatalf("Erro ao derivar chave com senha incorreta: %v", err)
		}
		if bytes.Equal(key1, key3) {
			t.Error("Chaves derivadas com senhas diferentes não deveriam corresponder")
		}
	})

	t.Run("Criptografia e descriptografia de canal", func(t *testing.T) {
		config := &EncryptionConfig{
			KeysDir: filepath.Join(testDir, "channel-crypto"),
		}

		service, err := NewEncryptionService(config)
		if err != nil {
			t.Fatalf("Erro ao criar serviço: %v", err)
		}

		// Derivar chave de canal
		channelName := "canal-crypto"
		password := "senha-canal"
		var salt []byte // Declarar salt para evitar erro de variável não utilizada

		channelKey, salt, err := service.DeriveChannelKey(channelName, password, nil)
		if err != nil {
			t.Fatalf("Erro ao derivar chave de canal: %v", err)
		}
		_ = salt // Usar salt para evitar erro de variável não utilizada

		// Dados para criptografar
		plaintext := []byte("Mensagem para o canal")

		// Criptografar dados
		encrypted, nonce, err := service.EncryptWithKey(plaintext, channelKey)
		if err != nil {
			t.Fatalf("Erro ao criptografar dados de canal: %v", err)
		}

		// Descriptografar dados
		decrypted, err := service.DecryptWithKey(encrypted, channelKey, nonce)
		if err != nil {
			t.Fatalf("Erro ao descriptografar dados de canal: %v", err)
		}

		// Verificar se os dados descriptografados correspondem ao original
		if !bytes.Equal(plaintext, decrypted) {
			t.Error("Dados de canal descriptografados não correspondem ao original")
		}

		// Verificar falha com chave incorreta
		wrongKey := make([]byte, len(channelKey))
		copy(wrongKey, channelKey)
		wrongKey[0] ^= 0x01 // Alterar um bit

		_, err = service.DecryptWithKey(encrypted, wrongKey, nonce)
		if err == nil {
			t.Error("Descriptografia com chave incorreta deveria falhar")
		}
	})

	t.Run("Gerenciamento de chaves efêmeras", func(t *testing.T) {
		config := &EncryptionConfig{
			KeysDir: filepath.Join(testDir, "ephemeral"),
		}

		service, err := NewEncryptionService(config)
		if err != nil {
			t.Fatalf("Erro ao criar serviço: %v", err)
		}

		// Gerar chave efêmera para peer
		peerID := "peer-ephemeral"
		publicKey, _, err := box.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("Erro ao gerar chave efêmera: %v", err)
		}

		// Armazenar chave
		err = service.StoreEphemeralKey(peerID, publicKey[:])
		if err != nil {
			t.Fatalf("Erro ao armazenar chave efêmera: %v", err)
		}

		// Recuperar chave
		retrievedKey, exists := service.GetEphemeralKey(peerID)
		if !exists {
			t.Fatal("Chave efêmera não encontrada")
		}
		if !bytes.Equal(publicKey[:], retrievedKey) {
			t.Error("Chave efêmera recuperada não corresponde à original")
		}

		// Remover chave
		service.RemoveEphemeralKey(peerID)

		// Verificar se foi removida
		_, exists = service.GetEphemeralKey(peerID)
		if exists {
			t.Error("Chave efêmera deveria ter sido removida")
		}
	})

	t.Run("HKDF", func(t *testing.T) {
		config := &EncryptionConfig{
			KeysDir: filepath.Join(testDir, "hkdf"),
		}

		service, err := NewEncryptionService(config)
		if err != nil {
			t.Fatalf("Erro ao criar serviço: %v", err)
		}

		// Material de chave inicial
		ikm := []byte("material-chave-inicial")
		salt := []byte("salt-para-hkdf")
		info := []byte("contexto-adicional")

		// Derivar chaves
		key1, err := service.DeriveKeyHKDF(ikm, salt, info, 32)
		if err != nil {
			t.Fatalf("Erro ao derivar chave com HKDF: %v", err)
		}

		// Derivar novamente com os mesmos parâmetros
		key2, err := service.DeriveKeyHKDF(ikm, salt, info, 32)
		if err != nil {
			t.Fatalf("Erro ao derivar chave novamente: %v", err)
		}

		// Verificar se as chaves são iguais
		if !bytes.Equal(key1, key2) {
			t.Error("Chaves HKDF derivadas com os mesmos parâmetros não correspondem")
		}

		// Derivar com parâmetros diferentes
		key3, err := service.DeriveKeyHKDF(ikm, []byte("salt-diferente"), info, 32)
		if err != nil {
			t.Fatalf("Erro ao derivar chave com salt diferente: %v", err)
		}
		if bytes.Equal(key1, key3) {
			t.Error("Chaves HKDF derivadas com parâmetros diferentes não deveriam corresponder")
		}
	})
}
