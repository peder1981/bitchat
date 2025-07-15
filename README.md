# Bitchat Go

Bitchat Go é uma implementação em Go Lang do aplicativo de mensagens descentralizado peer-to-peer que funciona sobre redes mesh Bluetooth LE. Esta versão é compatível com múltiplas plataformas, incluindo Linux.

## Características

- **Rede Mesh Descentralizada**: Descoberta automática de peers e relay de mensagens multi-hop sobre Bluetooth LE
- **Criptografia Ponta-a-Ponta**: Troca de chaves X25519 + AES-256-GCM para mensagens privadas e canais
- **Canais de Chat**: Mensagens em grupo baseadas em tópicos com proteção opcional por senha
- **Store & Forward**: Mensagens em cache para peers offline e entregues quando reconectam
- **Privacidade Primeiro**: Sem contas, sem números de telefone, sem identificadores persistentes
- **Comandos Estilo IRC**: Interface familiar com `/join`, `/msg`, `/who`
- **Retenção de Mensagens**: Salvamento opcional de mensagens controlado por donos de canais
- **Aplicativo Multiplataforma**: Suporte nativo para Linux, macOS e outras plataformas
- **Cover Traffic**: Ofuscação de timing e mensagens falsas para maior privacidade
- **Wipe de Emergência**: Limpar instantaneamente todos os dados
- **Otimizações de Performance**: Compressão de mensagens LZ4, modos adaptativos de bateria e networking otimizado

## Requisitos

- Go 1.18 ou superior
- Bibliotecas de desenvolvimento Bluetooth (Linux: `bluez`, macOS: nativo)

## Instalação

```bash
go install github.com/permissionlesstech/bitchat/cmd/bitchat@latest
```

## Compilação a partir do código-fonte

```bash
git clone https://github.com/permissionlesstech/bitchat.git
cd bitchat
go build ./cmd/bitchat
```

## Uso

### Comandos Básicos

- `/j #canal` - Entrar ou criar um canal
- `/m @nome mensagem` - Enviar uma mensagem privada
- `/w` - Listar usuários online
- `/channels` - Mostrar todos os canais descobertos
- `/block @nome` - Bloquear um peer
- `/block` - Listar todos os peers bloqueados
- `/unblock @nome` - Desbloquear um peer
- `/clear` - Limpar mensagens do chat
- `/pass [senha]` - Definir/alterar senha do canal (apenas dono)
- `/transfer @nome` - Transferir propriedade do canal
- `/save` - Alternar retenção de mensagens para o canal (apenas dono)

## Segurança e Privacidade

- **Mensagens Privadas**: Troca de chaves X25519 + criptografia AES-256-GCM
- **Mensagens de Canal**: Derivação de senha Argon2id + AES-256-GCM
- **Assinaturas Digitais**: Ed25519 para autenticidade de mensagens
- **Forward Secrecy**: Novos pares de chaves gerados a cada sessão
- **Sem Registro**: Não requer contas, emails ou números de telefone
- **Efêmero por Padrão**: Mensagens existem apenas na memória do dispositivo
- **Cover Traffic**: Atrasos aleatórios e mensagens falsas previnem análise de tráfego
- **Wipe de Emergência**: Limpar instantaneamente todos os dados
- **Local-First**: Funciona completamente offline, sem servidores

## Licença

Este projeto é liberado para o domínio público. Veja o arquivo [LICENSE](LICENSE) para detalhes.
