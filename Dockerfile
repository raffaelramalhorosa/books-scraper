# =============================================================================
# Stage 1 — Builder
# Usa a imagem oficial do Go com Alpine para compilar o binário.
# Alpine reduz o tamanho da imagem (~5 MB) e mantém as ferramentas de build.
# =============================================================================
FROM golang:1.23-alpine AS builder

# Instala certificados TLS necessários para chamadas HTTPS (Gemini API, books.toscrape.com).
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copia manifesto de dependências primeiro para aproveitar o cache de camadas do Docker.
# Se go.mod/go.sum não mudam, o `go mod download` não re-executa.
COPY go.mod go.sum ./
RUN go mod download

# Copia o restante do código-fonte.
COPY . .

# Compila o binário com:
#   CGO_ENABLED=0  → binário estático, sem dependência de libc (necessário para Alpine final)
#   -ldflags="-s -w" → remove tabela de símbolos e informações de debug (~30% menor)
#   -trimpath        → remove caminhos absolutos do host (segurança e reprodutibilidade)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /scraper \
    ./cmd/scraper

# =============================================================================
# Stage 2 — Final
# Imagem mínima de runtime. Não contém compilador, módulos Go nem ferramentas.
# Tamanho final estimado: ~15 MB (vs ~800 MB com a imagem de build completa).
# =============================================================================
FROM alpine:3.20

# Certificados TLS copiados do builder para manter chamadas HTTPS funcionando.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Cria um usuário não-root (princípio de menor privilégio).
# Se o container for comprometido, o atacante não terá acesso root ao host.
RUN addgroup -S scraper && adduser -S scraper -G scraper

WORKDIR /app

# Diretório de saída pré-criado com permissão do usuário não-root.
RUN mkdir -p output && chown scraper:scraper output

# Copia apenas o binário compilado do stage anterior.
COPY --from=builder /scraper /app/scraper

# Alterna para usuário não-root antes de qualquer execução.
USER scraper

# Porta exposta para futuras extensões (ex: endpoint de health check / métricas).
# O scraper atual é um CLI sem servidor HTTP, mas a exposição documenta a intenção.
EXPOSE 8080

ENTRYPOINT ["/app/scraper"]
