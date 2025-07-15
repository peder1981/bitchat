#!/bin/bash

# Script para compilar o BitChat para múltiplas plataformas

# Criar diretório de saída
mkdir -p dist

# Versão do aplicativo
VERSION="0.1.0"
COMMIT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "dev")
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Flags de compilação comuns
LDFLAGS="-X main.Version=$VERSION -X main.CommitHash=$COMMIT_HASH -X main.BuildDate=$BUILD_DATE"

echo "Compilando BitChat v$VERSION ($COMMIT_HASH) - $BUILD_DATE"
echo "----------------------------------------"

build_platform() {
    local os=$1
    local arch=$2
    local ext=""
    
    if [ "$os" == "windows" ]; then
        ext=".exe"
    fi
    
    echo "Compilando para $os ($arch)..."
    
    GOOS=$os GOARCH=$arch go build -tags "$os" -ldflags "$LDFLAGS" -o dist/bitchat-$os-$arch$ext ./cmd/bitchat
    
    if [ $? -eq 0 ]; then
        echo "✓ Compilado: dist/bitchat-$os-$arch$ext"
        return 0
    else
        echo "✗ Falha na compilação para $os ($arch)"
        return 1
    fi
}

# Compilar para Linux (amd64)
build_platform linux amd64

# Compilar para Linux (arm64)
build_platform linux arm64 || echo "Compilação para Linux (arm64) não suportada neste ambiente"

# Compilar para macOS (amd64)
build_platform darwin amd64 || echo "Compilação para macOS (amd64) não suportada neste ambiente"

# Compilar para macOS (arm64)
build_platform darwin arm64 || echo "Compilação para macOS (arm64) não suportada neste ambiente"

# Compilar para Windows (amd64)
build_platform windows amd64 || echo "Compilação para Windows (amd64) não suportada neste ambiente"

echo "----------------------------------------"
echo "Compilação concluída! Binários disponíveis no diretório 'dist':"
ls -lh dist/

echo "----------------------------------------"
echo "Verificando tamanho dos binários:"
du -h dist/*
