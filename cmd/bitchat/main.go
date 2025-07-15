package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/permissionlesstech/bitchat/internal/bluetooth"
	"github.com/permissionlesstech/bitchat/internal/crypto"
	"github.com/permissionlesstech/bitchat/internal/protocol"
	"github.com/permissionlesstech/bitchat/pkg/utils"
)

const (
	AppVersion = "0.1.0"
)

// Opções de configuração
type Config struct {
	DeviceName       string
	DataDir          string
	BatteryMode      int
	CoverTraffic     bool
	Debug            bool
}

// Estado global do aplicativo
type AppState struct {
	Config           *Config
	EncryptionService *crypto.EncryptionService
	MeshService      *bluetooth.BluetoothMeshService
	CurrentChannel   string
	ActivePeers      map[string]string // peerID -> nickname
	BlockedPeers     map[string]bool
	MessageHistory   map[string][]*protocol.BitchatMessage // canal -> mensagens
	PrivateMessages  map[string][]*protocol.BitchatMessage // peerID -> mensagens
	Running          bool
}

// Implementação de MeshDelegate
type MeshDelegateImpl struct {
	AppState *AppState
}

// OnPeerDiscovered é chamado quando um novo peer é descoberto
func (md *MeshDelegateImpl) OnPeerDiscovered(peerID string, name string) {
	md.AppState.ActivePeers[peerID] = name
	fmt.Printf("Peer descoberto: %s (%s)\n", name, peerID)
}

// OnPeerLost é chamado quando um peer não é mais visível
func (md *MeshDelegateImpl) OnPeerLost(peerID string) {
	if name, ok := md.AppState.ActivePeers[peerID]; ok {
		fmt.Printf("Peer perdido: %s (%s)\n", name, peerID)
		delete(md.AppState.ActivePeers, peerID)
	}
}

// OnMessageReceived é chamado quando uma nova mensagem é recebida
func (md *MeshDelegateImpl) OnMessageReceived(message *protocol.BitchatMessage) {
	// Verificar se o remetente está bloqueado
	if md.AppState.BlockedPeers[message.SenderPeerID] {
		return
	}

	// Processar a mensagem
	if message.IsPrivate {
		// Mensagem privada
		if _, ok := md.AppState.PrivateMessages[message.SenderPeerID]; !ok {
			md.AppState.PrivateMessages[message.SenderPeerID] = make([]*protocol.BitchatMessage, 0)
		}
		md.AppState.PrivateMessages[message.SenderPeerID] = append(
			md.AppState.PrivateMessages[message.SenderPeerID], message)
		
		fmt.Printf("[Privado de %s]: %s\n", message.Sender, message.Content)
	} else if message.Channel != "" {
		// Mensagem de canal
		if message.Channel == md.AppState.CurrentChannel {
			fmt.Printf("[%s] %s: %s\n", message.Channel, message.Sender, message.Content)
		}
		
		if _, ok := md.AppState.MessageHistory[message.Channel]; !ok {
			md.AppState.MessageHistory[message.Channel] = make([]*protocol.BitchatMessage, 0)
		}
		md.AppState.MessageHistory[message.Channel] = append(
			md.AppState.MessageHistory[message.Channel], message)
	} else {
		// Mensagem broadcast
		fmt.Printf("[Broadcast] %s: %s\n", message.Sender, message.Content)
	}
}

// OnMessageDeliveryChanged é chamado quando o status de entrega de uma mensagem muda
func (md *MeshDelegateImpl) OnMessageDeliveryChanged(messageID string, status protocol.DeliveryStatus, info *protocol.DeliveryInfo) {
	// Implementação básica - apenas log
	statusText := "desconhecido"
	switch status {
	case protocol.DeliveryStatusSending:
		statusText = "enviando"
	case protocol.DeliveryStatusSent:
		statusText = "enviado"
	case protocol.DeliveryStatusDelivered:
		statusText = "entregue"
	case protocol.DeliveryStatusRead:
		statusText = "lido"
	case protocol.DeliveryStatusFailed:
		statusText = "falhou"
	case protocol.DeliveryStatusPartiallyDelivered:
		statusText = "parcialmente entregue"
	}
	
	if md.AppState.Config.Debug {
		fmt.Printf("Status da mensagem %s: %s\n", messageID, statusText)
	}
}

func main() {
	// Configuração via flags
	config := &Config{}
	
	flag.StringVar(&config.DeviceName, "name", "", "Nome do dispositivo (se não definido, será gerado)")
	flag.StringVar(&config.DataDir, "data", "", "Diretório para dados persistentes (padrão: ~/.bitchat)")
	flag.BoolVar(&config.CoverTraffic, "cover", true, "Ativar tráfego de cobertura para privacidade")
	flag.BoolVar(&config.Debug, "debug", false, "Ativar modo de depuração")
	flag.Parse()
	
	// Configurar diretório de dados
	if config.DataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Println("Erro ao obter diretório home:", err)
			os.Exit(1)
		}
		config.DataDir = filepath.Join(homeDir, ".bitchat")
	}
	
	// Criar diretório de dados se não existir
	if err := os.MkdirAll(config.DataDir, 0700); err != nil {
		fmt.Println("Erro ao criar diretório de dados:", err)
		os.Exit(1)
	}
	
	// Gerar nome do dispositivo se não fornecido
	if config.DeviceName == "" {
		config.DeviceName = fmt.Sprintf("user-%x", utils.GenerateRandomID(4))
	}
	
	// Inicializar estado do aplicativo
	appState := &AppState{
		Config:          config,
		ActivePeers:     make(map[string]string),
		BlockedPeers:    make(map[string]bool),
		MessageHistory:  make(map[string][]*protocol.BitchatMessage),
		PrivateMessages: make(map[string][]*protocol.BitchatMessage),
		Running:         true,
	}
	
	// Carregar ou criar chave de identidade
	identityKeyPath := filepath.Join(config.DataDir, "identity.key")
	var identityKey []byte
	
	if _, err := os.Stat(identityKeyPath); err == nil {
		// Arquivo existe, carregar chave
		identityKey, err = os.ReadFile(identityKeyPath)
		if err != nil {
			fmt.Println("Erro ao ler chave de identidade:", err)
			os.Exit(1)
		}
	}
	
	// Inicializar serviço de criptografia
	cryptoConfig := &crypto.EncryptionConfig{
		KeysDir: filepath.Join(config.DataDir, "keys"),
		UseEphemeralOnly: false,
	}
	
	// Se temos uma chave de identidade, configurar o caminho para ela
	if identityKey != nil {
		keyPath := filepath.Join(config.DataDir, "identity_key")
		if err := os.WriteFile(keyPath, identityKey, 0600); err != nil {
			fmt.Println("Erro ao salvar chave de identidade temporária:", err)
			os.Exit(1)
		}
		cryptoConfig.KeyStorePath = keyPath
	}
	
	encryptionService, err := crypto.NewEncryptionService(cryptoConfig)
	if err != nil {
		fmt.Println("Erro ao inicializar serviço de criptografia:", err)
		os.Exit(1)
	}
	appState.EncryptionService = encryptionService
	
	// Salvar nova chave de identidade se foi criada
	if identityKey == nil {
		newIdentityKey := encryptionService.GetIdentityKey()
		if err := os.WriteFile(identityKeyPath, newIdentityKey, 0600); err != nil {
			fmt.Println("Aviso: Não foi possível salvar chave de identidade:", err)
		}
	}
	
	// Gerar ID do dispositivo
	deviceID := utils.GenerateRandomID(8)
	
	// Inicializar serviço Bluetooth Mesh
	meshService := bluetooth.NewBluetoothMeshService(
		deviceID,
		config.DeviceName,
		encryptionService,
	)
	appState.MeshService = meshService
	
	// Configurar delegate
	meshDelegate := &MeshDelegateImpl{AppState: appState}
	meshService.SetDelegate(meshDelegate)
	
	// Configurar opções
	meshService.SetCoverTraffic(config.CoverTraffic)
	
	// Iniciar serviço mesh
	if err := meshService.Start(); err != nil {
		fmt.Println("Erro ao iniciar serviço mesh:", err)
		os.Exit(1)
	}
	
	// Exibir informações iniciais
	fmt.Println("Bitchat", AppVersion)
	fmt.Println("Nome do dispositivo:", config.DeviceName)
	fmt.Println("ID do dispositivo:", fmt.Sprintf("%x", deviceID))
	fmt.Println("Diretório de dados:", config.DataDir)
	fmt.Println("Tráfego de cobertura:", config.CoverTraffic)
	fmt.Println("Digite /help para ajuda")
	
	// Configurar captura de sinais para encerramento limpo
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Iniciar loop de entrada do usuário em uma goroutine
	go inputLoop(appState)
	
	// Aguardar sinal de encerramento
	<-sigChan
	fmt.Println("\nEncerrando...")
	
	// Parar serviços
	appState.Running = false
	meshService.Stop()
	
	fmt.Println("Bitchat encerrado")
}

// inputLoop processa entrada do usuário
func inputLoop(appState *AppState) {
	scanner := bufio.NewScanner(os.Stdin)
	
	for appState.Running && scanner.Scan() {
		input := scanner.Text()
		processUserInput(input, appState)
	}
}

// processUserInput processa comandos e mensagens do usuário
func processUserInput(input string, appState *AppState) {
	if strings.TrimSpace(input) == "" {
		return
	}
	
	// Verificar se é um comando
	if strings.HasPrefix(input, "/") {
		parts := strings.SplitN(input, " ", 2)
		command := parts[0]
		args := ""
		if len(parts) > 1 {
			args = parts[1]
		}
		
		processCommand(command, args, appState)
	} else {
		// Mensagem normal para o canal atual
		if appState.CurrentChannel == "" {
			fmt.Println("Você não está em nenhum canal. Use /j #canal para entrar em um canal.")
			return
		}
		
		// Criar mensagem
		message := &protocol.BitchatMessage{
			Content: input,
			Channel: appState.CurrentChannel,
		}
		
		// Enviar mensagem
		messageID, err := appState.MeshService.SendMessage(message)
		if err != nil {
			fmt.Println("Erro ao enviar mensagem:", err)
			return
		}
		
		// Adicionar à história local
		if _, ok := appState.MessageHistory[appState.CurrentChannel]; !ok {
			appState.MessageHistory[appState.CurrentChannel] = make([]*protocol.BitchatMessage, 0)
		}
		
		// Adicionar informações locais
		message.ID = messageID
		message.Timestamp = uint64(time.Now().UnixMilli())
		message.Sender = appState.Config.DeviceName
		message.DeliveryStatus = protocol.DeliveryStatusSending
		
		appState.MessageHistory[appState.CurrentChannel] = append(
			appState.MessageHistory[appState.CurrentChannel], message)
	}
}

// processCommand processa comandos do usuário
func processCommand(command, args string, appState *AppState) {
	switch command {
	case "/j", "/join":
		if args == "" || !strings.HasPrefix(args, "#") {
			fmt.Println("Uso: /j #canal")
			return
		}
		
		channel := args
		appState.CurrentChannel = channel
		fmt.Printf("Entrando no canal %s\n", channel)
		
		// Exibir histórico do canal se disponível
		if messages, ok := appState.MessageHistory[channel]; ok && len(messages) > 0 {
			fmt.Printf("--- Histórico do canal %s ---\n", channel)
			for _, msg := range messages {
				fmt.Printf("[%s] %s: %s\n", 
					time.Unix(0, int64(msg.Timestamp)*int64(time.Millisecond)).Format("15:04:05"),
					msg.Sender, 
					msg.Content)
			}
			fmt.Println("--- Fim do histórico ---")
		}
		
	case "/m", "/msg":
		parts := strings.SplitN(args, " ", 2)
		if len(parts) < 2 || !strings.HasPrefix(parts[0], "@") {
			fmt.Println("Uso: /m @usuario mensagem")
			return
		}
		
		recipient := parts[0][1:] // Remover @
		content := parts[1]
		
		// Buscar peer pelo nickname
		var recipientPeerID string
		for id, name := range appState.ActivePeers {
			if name == recipient {
				recipientPeerID = id
				break
			}
		}
		
		if recipientPeerID == "" {
			fmt.Printf("Usuário %s não encontrado\n", recipient)
			return
		}
		
		// Criar mensagem privada
		message := &protocol.BitchatMessage{
			Content:          content,
			IsPrivate:        true,
			RecipientNickname: recipient,
		}
		
		// Enviar mensagem
		messageID, err := appState.MeshService.SendMessage(message)
		if err != nil {
			fmt.Println("Erro ao enviar mensagem privada:", err)
			return
		}
		
		// Adicionar à história local
		if _, ok := appState.PrivateMessages[recipientPeerID]; !ok {
			appState.PrivateMessages[recipientPeerID] = make([]*protocol.BitchatMessage, 0)
		}
		
		// Adicionar informações locais
		message.ID = messageID
		message.Timestamp = uint64(time.Now().UnixMilli())
		message.Sender = appState.Config.DeviceName
		message.DeliveryStatus = protocol.DeliveryStatusSending
		
		appState.PrivateMessages[recipientPeerID] = append(
			appState.PrivateMessages[recipientPeerID], message)
		
		fmt.Printf("[Privado para %s]: %s\n", recipient, content)
		
	case "/w", "/who":
		fmt.Println("Peers online:")
		if len(appState.ActivePeers) == 0 {
			fmt.Println("  Nenhum peer encontrado")
		} else {
			for id, name := range appState.ActivePeers {
				fmt.Printf("  %s (%s)\n", name, id)
			}
		}
		
	case "/channels":
		fmt.Println("Canais ativos:")
		if len(appState.MessageHistory) == 0 {
			fmt.Println("  Nenhum canal ativo")
		} else {
			for channel := range appState.MessageHistory {
				fmt.Printf("  %s\n", channel)
			}
		}
		
	case "/block":
		if args == "" {
			// Listar peers bloqueados
			fmt.Println("Peers bloqueados:")
			if len(appState.BlockedPeers) == 0 {
				fmt.Println("  Nenhum peer bloqueado")
			} else {
				for id := range appState.BlockedPeers {
					name := "desconhecido"
					if n, ok := appState.ActivePeers[id]; ok {
						name = n
					}
					fmt.Printf("  %s (%s)\n", name, id)
				}
			}
		} else if !strings.HasPrefix(args, "@") {
			fmt.Println("Uso: /block @usuario")
		} else {
			// Bloquear peer
			username := args[1:] // Remover @
			
			// Buscar peer pelo nickname
			var peerID string
			for id, name := range appState.ActivePeers {
				if name == username {
					peerID = id
					break
				}
			}
			
			if peerID == "" {
				fmt.Printf("Usuário %s não encontrado\n", username)
				return
			}
			
			appState.BlockedPeers[peerID] = true
			fmt.Printf("Usuário %s bloqueado\n", username)
		}
		
	case "/unblock":
		if args == "" || !strings.HasPrefix(args, "@") {
			fmt.Println("Uso: /unblock @usuario")
			return
		}
		
		username := args[1:] // Remover @
		
		// Buscar peer pelo nickname
		var peerID string
		for id, name := range appState.ActivePeers {
			if name == username {
				peerID = id
				break
			}
		}
		
		if peerID == "" {
			fmt.Printf("Usuário %s não encontrado\n", username)
			return
		}
		
		delete(appState.BlockedPeers, peerID)
		fmt.Printf("Usuário %s desbloqueado\n", username)
		
	case "/clear":
		if appState.CurrentChannel != "" {
			// Limpar histórico do canal atual
			delete(appState.MessageHistory, appState.CurrentChannel)
			fmt.Printf("Histórico do canal %s limpo\n", appState.CurrentChannel)
		} else {
			fmt.Println("Você não está em nenhum canal")
		}
		
	case "/battery":
		if args == "" {
			fmt.Println("Uso: /battery [normal|low|ultralow]")
			return
		}
		
		mode := strings.ToLower(args)
		var batteryMode int
		
		switch mode {
		case "normal":
			batteryMode = bluetooth.BatteryModeNormal
		case "low":
			batteryMode = bluetooth.BatteryModeLow
		case "ultralow":
			batteryMode = bluetooth.BatteryModeUltraLow
		default:
			fmt.Println("Modo inválido. Use: normal, low ou ultralow")
			return
		}
		
		appState.MeshService.SetBatteryMode(batteryMode)
		fmt.Printf("Modo de bateria alterado para: %s\n", mode)
		
	case "/cover":
		if args == "" {
			fmt.Println("Uso: /cover [on|off]")
			return
		}
		
		enabled := strings.ToLower(args) == "on"
		appState.MeshService.SetCoverTraffic(enabled)
		
		if enabled {
			fmt.Println("Tráfego de cobertura ativado")
		} else {
			fmt.Println("Tráfego de cobertura desativado")
		}
		
	case "/help":
		fmt.Println("Comandos disponíveis:")
		fmt.Println("  /j #canal - Entrar ou criar um canal")
		fmt.Println("  /m @nome mensagem - Enviar uma mensagem privada")
		fmt.Println("  /w - Listar usuários online")
		fmt.Println("  /channels - Mostrar todos os canais descobertos")
		fmt.Println("  /block @nome - Bloquear um peer")
		fmt.Println("  /block - Listar todos os peers bloqueados")
		fmt.Println("  /unblock @nome - Desbloquear um peer")
		fmt.Println("  /clear - Limpar mensagens do chat atual")
		fmt.Println("  /battery [normal|low|ultralow] - Definir modo de economia de bateria")
		fmt.Println("  /cover [on|off] - Ativar/desativar tráfego de cobertura")
		fmt.Println("  /help - Mostrar esta ajuda")
		fmt.Println("  /quit - Sair do aplicativo")
		
	case "/quit", "/exit":
		fmt.Println("Saindo...")
		appState.Running = false
		os.Exit(0)
		
	default:
		fmt.Printf("Comando desconhecido: %s\nDigite /help para ajuda\n", command)
	}
}
