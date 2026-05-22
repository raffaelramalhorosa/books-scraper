# books-scraper

Scraper em Go para [books.toscrape.com](https://books.toscrape.com) — coleta todos os ~1000 livros do site (50 páginas de listagem + página de detalhe de cada livro), enriquece as descrições com Gemini Flash e persiste os dados em JSON, CSV e PostgreSQL.

Desenvolvido como desafio técnico para a trilha **Crawler/RPA & IA** da IN8 Holding / Devnology.

---

## Sumário

- [Como rodar localmente](#como-rodar-localmente)
- [Schema dos dados](#schema-dos-dados)
- [Como o pipeline funciona](#como-o-pipeline-funciona)
- [Decisões técnicas](#decisões-técnicas)
- [O que faria diferente com mais tempo](#o-que-faria-diferente-com-mais-tempo)
- [Como usei IA no desafio](#como-usei-ia-no-desafio)

---

## Como rodar localmente

### Pré-requisitos

- Go 1.23+
- (Opcional) Docker + Docker Compose
- (Opcional) Chave de API do Gemini Flash

### Sem Docker

```bash
# Clone o repositório
git clone <repo-url>
cd books-scraper

# Instale as dependências
go mod download

# Rode com configuração padrão (sem Gemini, sem PostgreSQL)
go run ./cmd/scraper \
  --pages=5 \
  --delay=300ms \
  --workers=2 \
  --formats=json,csv \
  --output=./output

# Com Gemini Flash habilitado
GEMINI_API_KEY=sua_chave go run ./cmd/scraper \
  --pages=5 \
  --gemini \
  --formats=json,csv \
  --output=./output

# Com PostgreSQL
go run ./cmd/scraper \
  --pages=50 \
  --formats=json,csv,postgres \
  --db-url="postgres://user:pass@localhost:5432/bookscraper?sslmode=disable" \
  --output=./output
```

#### Flags disponíveis

| Flag | Padrão | Descrição |
|------|--------|-----------|
| `--pages` | `50` | Máximo de páginas de listagem a raspar (0 = sem limite) |
| `--delay` | `300ms` | Delay entre requests por domínio |
| `--workers` | `2` | Goroutines paralelas para páginas de detalhe |
| `--output` | `./output` | Diretório de saída dos arquivos |
| `--formats` | `json,csv` | Formatos de saída: `json`, `csv`, `postgres` |
| `--db-url` | `""` | DSN do PostgreSQL (obrigatório se `postgres` está nos formatos) |
| `--gemini-key` | `""` | Chave da API Gemini (ou use env `GEMINI_API_KEY`) |
| `--gemini` | `false` | Habilita análise de descrições com Gemini Flash |

### Com Docker Compose (scraper + PostgreSQL)

```bash
# Configure a chave Gemini (opcional)
export GEMINI_API_KEY=sua_chave

# Suba todos os serviços
docker compose up --build

# Os arquivos gerados ficam em ./output/
```

O `docker-compose.yml` inclui um health check no PostgreSQL: o scraper só inicia após o banco estar pronto para aceitar conexões.

### Build da imagem manualmente

```bash
docker build -t books-scraper .
docker run --rm \
  -v $(pwd)/output:/app/output \
  -e GEMINI_API_KEY=sua_chave \
  books-scraper \
  --pages=5 --gemini --formats=json,csv
```

---

## Schema dos dados

### JSON — `output/books.json`

```json
[
  {
    "title": "A Light in the Attic",
    "url": "https://books.toscrape.com/catalogue/a-light-in-the-attic_1000/index.html",
    "image_url": "https://books.toscrape.com/media/cache/.../a-light-in-the-attic.jpg",
    "category": "Poetry",
    "upc": "a897fe39b1053632",
    "rating": 3,
    "price": 51.77,
    "price_excl_tax": 51.77,
    "price_incl_tax": 51.77,
    "tax": 0.00,
    "availability": "In stock",
    "stock_count": 22,
    "num_reviews": 0,
    "description": "It's hard to imagine...",
    "ai_analysis": {
      "genres": ["Poetry", "Children's Literature"],
      "themes": ["Imagination", "Childhood", "Wonder"],
      "target_audience": "Children and adults who enjoy whimsical poetry",
      "reading_level": "Beginner",
      "mood": "Playful",
      "summary_pt": "Uma coleção de poemas que celebra a imaginação infantil..."
    }
  }
]
```

> `ai_analysis` só aparece quando `--gemini` está habilitado e a descrição não está vazia.

### CSV — `output/books.csv`

| Coluna | Tipo | Descrição |
|--------|------|-----------|
| `title` | string | Título do livro |
| `url` | string | URL da página de detalhe |
| `image_url` | string | URL da capa |
| `category` | string | Categoria (via breadcrumb) |
| `upc` | string | Universal Product Code |
| `rating` | int (1-5) | Avaliação numérica |
| `price` | decimal | Preço em libras |
| `price_excl_tax` | decimal | Preço sem imposto |
| `price_incl_tax` | decimal | Preço com imposto |
| `tax` | decimal | Valor do imposto |
| `availability` | string | `In stock` / `Out of stock` |
| `stock_count` | int | Quantidade em estoque |
| `num_reviews` | int | Número de avaliações |
| `description` | string | Descrição completa |
| `ai_genres` | string | Gêneros separados por `;` |
| `ai_themes` | string | Temas separados por `;` |
| `ai_target_audience` | string | Público-alvo |
| `ai_reading_level` | string | `Beginner` / `Intermediate` / `Advanced` |
| `ai_mood` | string | Mood em uma palavra |
| `ai_summary_pt` | string | Resumo em português |

### PostgreSQL — tabela `books`

```sql
CREATE TABLE books (
    id              SERIAL PRIMARY KEY,
    title           TEXT        NOT NULL,
    url             TEXT        UNIQUE NOT NULL,
    image_url       TEXT,
    category        TEXT,
    upc             TEXT,
    rating          SMALLINT,
    price           NUMERIC(10,2),
    price_excl_tax  NUMERIC(10,2),
    price_incl_tax  NUMERIC(10,2),
    tax             NUMERIC(10,2),
    availability    TEXT,
    stock_count     INT,
    num_reviews     INT,
    description     TEXT,
    ai_genres       TEXT[],
    ai_themes       TEXT[],
    ai_target_audience TEXT,
    ai_reading_level   TEXT,
    ai_mood            TEXT,
    ai_summary_pt      TEXT,
    created_at      TIMESTAMPTZ DEFAULT now()
);
```

A URL é `UNIQUE` — re-execuções fazem upsert (não duplicam dados).

---

## Como o pipeline funciona

```
commit → lint → test → build → deploy (apenas main)
```

### `lint` — Análise estática
- Roda `golangci-lint run ./...` com o conjunto de linters configurado em `.golangci.yml`
- Linters ativos: `errcheck`, `gosec`, `staticcheck`, `govet`, `bodyclose`, entre outros
- **Falha o pipeline** se houver qualquer erro — garante qualidade antes de compilar
- Cache de módulos Go evita re-download em cada pipeline

### `test` — Testes unitários
- Roda `go test -v -race -coverprofile=coverage.out ./...`
- `-race` detecta race conditions em código concorrente
- Gera artefato de cobertura de código (relatório no GitLab)
- **Falha o pipeline** se qualquer teste falhar

### `build` — Build Docker + Push
- Usa `docker:26-dind` para builds dentro do CI
- Constrói a imagem multi-stage e faz push para o GitLab Container Registry
- Tag baseada em `$CI_COMMIT_SHA` para rastreabilidade total
- Na branch `main`, também cria a tag `latest`
- Variáveis `$CI_REGISTRY_USER`, `$CI_REGISTRY_PASSWORD`, `$CI_REGISTRY` são automáticas no GitLab

### `deploy` — Deploy simulado (somente `main`)
- Exibe os comandos `aws ecs` que seriam executados em produção
- Em produção real: precisaria das variáveis `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` configuradas como CI/CD secrets no GitLab
- Usa `rules: if: '$CI_COMMIT_BRANCH == "main"'` — mais flexível que `only: - main`

---

## Decisões técnicas

### Go + Colly
Go foi escolhido pela combinação de tipagem estática (erros em compile-time), goroutines nativas para scraping concorrente e binário estático sem runtime externo. Colly foi preferido ao `net/http` + `goquery` puro porque gerencia rate limiting, respeito ao `robots.txt`, pool de goroutines e callbacks declarativos de forma nativa — eliminar boilerplate reduz surface area para bugs.

### Dois coletores Colly separados
Um coletor para páginas de listagem (paginação) e outro para páginas de detalhe. O coletor de listagem opera com paralelismo 1 (preserva ordem das páginas), enquanto o de detalhes opera com `--workers` goroutines (acelera o gargalo real do scraping).

### Três formatos de saída
JSON preserva a estrutura hierárquica (incluindo `ai_analysis`), CSV facilita análise ad-hoc em Excel/Sheets sem escrever código, e PostgreSQL permite queries complexas e persistência durável. Cada formato serve um caso de uso diferente.

### Upsert via tabela staging no PostgreSQL
Em vez de `INSERT ... ON CONFLICT` row-a-row, a estratégia usa `COPY` para uma tabela temporária (o caminho de bulk insert mais rápido no PostgreSQL) e um único `INSERT ... SELECT ... ON CONFLICT` ao final. Para ~1000 livros, isso é significativamente mais rápido que 1000 statements individuais.

### Gemini Flash (não Gemini Pro/Ultra)
Flash tem latência de ~1-2s por request e custo muito menor. Para extração estruturada de campos literários a partir de descrições curtas, Flash é suficiente. Pro/Ultra seria desperdício de custo e latência neste contexto.

### `ResponseMIMEType: "application/json"` no Gemini
Força o modelo a retornar JSON válido diretamente, sem markdown fences. Mais confiável que depender de parsing de texto livre.

### Dockerfile multi-stage + Alpine
A imagem final tem ~15 MB em vez de ~800 MB. O compilador Go e todos os módulos de build ficam apenas no stage 1. Alpine reduz a superfície de ataque (menos pacotes = menos vulnerabilidades).

### Usuário não-root no container
Princípio de menor privilégio. Se o container for comprometido via vulnerabilidade no código, o atacante terá permissões limitadas sem escalada de privilégio automática.

---

## O que faria diferente com mais tempo

**Anti-bot e robustez:**
Rotação de proxies via SOCKS5, user-agent rotation mais agressiva, detecção de CAPTCHAs e fallback para headless browser (`chromedp`) em páginas que bloqueiam scrapers simples.

**Observabilidade:**
Structured logging com `log/slog` já está implementado, mas adicionaria métricas Prometheus (requests/s, erros, latência por página), tracing com OpenTelemetry e um dashboard Grafana no docker-compose.

**Scraping incremental:**
Um scheduler (cron ou Temporal) para re-scraping periódico que só atualiza livros alterados (comparando UPC + preço), evitando re-coletar dados já existentes.

**Retry com backoff exponencial:**
Para requests que falham por rate limiting do servidor alvo. Colly suporta isso via `colly.RetryHTTPCodes` + middleware customizado.

**IA mais rica:**
Gerar embeddings das descrições com `text-embedding-004` do Google e armazená-los como `vector` no PostgreSQL (extensão `pgvector`) para habilitar busca semântica ("livros parecidos com X").

**Pipeline CI:**
Adicionar stage de `security-scan` com `trivy` para vulnerabilidades na imagem Docker e `govulncheck` para vulnerabilidades nas dependências Go.

---

## Como usei IA no desafio

| Etapa | Ferramenta | O que fiz | Resultado |
|-------|-----------|-----------|-----------|
| Planejamento da arquitetura | Claude Code (Sonnet 4.6) | Discuti as decisões de design antes de escrever uma linha de código: qual biblioteca de scraping, estrutura de pastas, quais campos extrair, como integrar o Gemini | Arquitetura `cmd/internal`, justificativas técnicas documentadas no README |
| Implementação | Claude Code (Sonnet 4.6) | Geração do código Go completo com supervisão: revisei cada arquivo gerado, corrigi edge cases e ajustei a lógica de upsert do PostgreSQL | Código funcional com build e testes passando |
| Integração Gemini | Claude Code + Gemini Flash (runtime) | Claude ajudou a escrever o prompt de análise e o cliente Go; Gemini Flash roda em produção para enriquecer as descrições dos livros | Extração estruturada de gêneros, temas, público-alvo, mood e resumo em PT |
| Infraestrutura | Claude Code | Geração do Dockerfile multi-stage, docker-compose e .gitlab-ci.yml com justificativas para cada escolha | Arquivos prontos para uso com comentários explicativos |
| Documentação | Claude Code | Estrutura do README, schema das tabelas, explicação do pipeline | README em markdown com todas as seções do enunciado |

**O que funcionou bem:** Usar IA para gerar o boilerplate inicial e as estruturas de dados. A capacidade de explicar "por que" cada escolha foi feita — útil direto para a entrevista.

**O que exigiu supervisão manual:** A lógica de dois coletores Colly separados (listagem vs. detalhe) precisou de ajuste manual na passagem de contexto via `colly.Context`. A estratégia de bulk upsert no PostgreSQL também foi refinada manualmente após o design inicial.

**Transparência:** Aproximadamente 80% do código foi gerado com auxílio de IA e 20% ajustado ou escrito manualmente após revisão. Todo o raciocínio técnico por trás das decisões foi desenvolvido em conjunto entre eu e a IA — não foi "aceitar sugestão cegamente".
