# Customizações e Ajustes do Fork

Este documento descreve todas as customizações feitas neste fork do projeto Minha Receita para atender necessidades específicas e adaptar à nova infraestrutura da Receita Federal.

## 📋 Índice

1. [Contexto e Motivação](#contexto-e-motivação)
2. [Mudanças na Infraestrutura da Receita Federal](#mudanças-na-infraestrutura-da-receita-federal)
3. [Arquivos de Regime Tributário Opcionais](#arquivos-de-regime-tributário-opcionais)
4. [Melhorias no Docker Compose](#melhorias-no-docker-compose)
5. [Limite de Registros para Espaço em Disco](#limite-de-registros-para-espaço-em-disco)
6. [Documentação Criada](#documentação-criada)
7. [Resumo Técnico](#resumo-técnico)

---

## Contexto e Motivação

Em fevereiro de 2026, a Receita Federal do Brasil migrou completamente sua infraestrutura de dados abertos de CNPJ. O projeto original estava quebrando com erros 404, e havia necessidade de adaptações para ambientes com espaço em disco limitado.

### Problemas Identificados

1. ❌ Downloads falhando com erro 404
2. ❌ Aplicação iniciando antes do banco de dados estar pronto
3. ❌ Arquivos de regime tributário não encontrados causando falha total
4. ❌ Impossibilidade de testar com datasets menores para ambientes com pouco espaço

---

## Mudanças na Infraestrutura da Receita Federal

### O Problema

**Infraestrutura Antiga (não funciona mais):**
```
URL: https://arquivos.receitafederal.gov.br/dados/cnpj/dados_abertos_cnpj/
Tipo: Listagem de diretórios Apache (HTML simples)
Acesso: Público direto via HTTP
Estrutura: URLs diretas para arquivos
```

**Nova Infraestrutura (SERPRO+/Nextcloud):**
```
URL Base: https://arquivos.receitafederal.gov.br/public.php/webdav
Tipo: Sistema Nextcloud com WebDAV
Acesso: Requer autenticação HTTP Basic
Token Público: gn672Ad4CF8N6TK
Estrutura: Pastas mensais organizadas
```

### A Solução Implementada

#### 1. `download/federal_revenue.go` - Reescrita Completa

**Novas Constantes:**
```go
// WebDAV endpoints
FederalRevenueBaseURL    = "https://arquivos.receitafederal.gov.br/public.php/webdav"
federalRevenueShareToken = "gn672Ad4CF8N6TK"
federalRevenueSourcePath = "/Dados/Cadastros/CNPJ"
federalRevenueTaxesPath  = "/Dados/Obrigacoes_Acessorias"
```

**Novas Estruturas para WebDAV:**
```go
type webDAVMultistatus struct {
    XMLName   xml.Name         `xml:"multistatus"`
    Responses []webDAVResponse `xml:"response"`
}

type webDAVResponse struct {
    Href     string         `xml:"href"`
    Propstat webDAVPropstat `xml:"propstat"`
}

type webDAVResourceType struct {
    Collection *struct{} `xml:"collection"`
}
```

**Novas Funções:**

1. **`webDAVList(path string)`** - Lista conteúdo de diretórios
   - Usa método HTTP PROPFIND
   - Autentica com token público
   - Retorna XML parseado com lista de arquivos/pastas

2. **`federalRevenueGetMostRecentDir(dirPath string)`** - Encontra pasta mais recente
   - Lista todas as pastas no formato YYYY-MM
   - Ordena e retorna a mais recente automaticamente
   - Exemplo: retorna `/Dados/Cadastros/CNPJ/2026-01`

3. **`listZipFiles(dirPath string)`** - Lista arquivos ZIP
   - Filtra apenas arquivos .zip
   - Gera URLs WebDAV com credenciais embutidas
   - Formato: `https://token:@host/path/file.zip`

**Funções Modificadas:**

1. **`federalRevenueGetURLs()`** - Agora usa WebDAV
   - Antes: Fazia scraping de HTML
   - Depois: Usa API WebDAV para listar arquivos
   - Inclui autenticação nas URLs

2. **`saveUpdatedAt()`** - Extrai data do nome da pasta
   - Antes: Procurava timestamps no HTML
   - Depois: Extrai YYYY-MM do nome da pasta mensal
   - Converte para YYYY-MM-01 para o campo updated_at

3. **`taxRegimeGetURLs()`** - Agora retorna lista vazia em caso de erro
   - Antes: Retornava erro parando todo o processo
   - Depois: Log de warning e continua sem os arquivos

**Imports Adicionados:**
```go
"encoding/xml"  // Para parser de respostas WebDAV
```

#### 2. `download/download.go` - Atualização de Constantes

**Mudanças:**
```go
// Substituído em todas as referências
federalRevenueURL → FederalRevenueBaseURL
```

Afetou as funções:
- `Download()`
- `URLs()`

### Resultado

✅ Downloads funcionando com nova infraestrutura SERPRO+  
✅ Detecção automática do mês mais recente  
✅ Autenticação transparente via URL  
✅ 100% compatível com código de processamento existente

---

## Arquivos de Regime Tributário Opcionais

### O Problema

Os arquivos de regime tributário não estão mais disponíveis na nova estrutura SERPRO+:
- Lucro Arbitrado.zip
- Lucro Presumido.zip
- Lucro Real.zip
- Imunes e Isentas.zip

A pasta `/Dados/Obrigacoes_Acessorias` existe mas está **vazia**. O código original falhava completamente quando esses arquivos não eram encontrados.

### A Solução Implementada

#### `transform/source.go` - Arquivos Opcionais

**Função `pathsForSource()` - Detecção de Arquivos Opcionais:**
```go
if len(ls) == 0 {
    // Tax regime files são opcionais
    if t == realProfit || t == presumedProfit || t == arbitratedProfit || t == noTaxes {
        slog.Warn("tax regime file not found (continuing without it)", 
                  "source", string(t), "directory", dir)
        return []string{}, nil  // Retorna vazio, não erro
    }
    return []string{}, fmt.Errorf("could not find any file matching %s in %s", string(t), dir)
}
```

**Função `newSource()` - Suporte a Sources Vazios:**
```go
// Se nenhum arquivo encontrado mas é source opcional
if len(ls) == 0 {
    s = source{kind: t, dir: d, files: []string{}, total: 0}
    if !done.Load() {
        ch <- nil  // Retorna source vazio sem erro
    }
    return
}
```

**Funções Modificadas para Suportar Sources Vazios:**

Todas essas funções agora verificam `len(s.files) == 0` e retornam sem processar:

1. `createReaders()` - Não cria readers para sources vazios
2. `close()` - Não tenta fechar readers inexistentes
3. `resetReaders()` - Pula se source vazio
4. `countLines()` - Retorna 0 se source vazio
5. `sendTo()` - Pula envio se source vazio

### Impacto

- ✅ Processo de transform completa com sucesso
- ✅ Warnings informativos no log
- ⚠️ Campo `regime_tributario` fica vazio/nulo nos registros
- 🔮 Quando/se a Receita disponibilizar os arquivos novamente, serão carregados automaticamente

---

## Melhorias no Docker Compose

### O Problema

A aplicação estava tentando conectar ao PostgreSQL enquanto ele ainda estava inicializando:
```
FATAL: the database system is starting up (SQLSTATE 57P03)
```

### A Solução Implementada

#### `docker-compose.yml` - Health Checks e Dependências

**Service `minha-receita` - Build Explícito:**
```yaml
minha-receita:
  build:
    context: .
    dockerfile: Dockerfile  # Agora explícito
  ports:
    - 8000:8000
  env_file:
    - .env
  volumes:
    - ./data:/mnt/data
  depends_on:
    postgres:
      condition: service_healthy  # Espera health check
    mongo:
      condition: service_healthy  # Espera health check
```

**Service `mongo` - Novo Health Check:**
```yaml
mongo:
  image: mongo:8.0-noble
  # ... configurações existentes ...
  healthcheck:
    test: ["CMD", "mongosh", "--eval", "db.adminCommand('ping')"]
    interval: 10s
    timeout: 5s
    retries: 5
```

### Resultado

- ✅ PostgreSQL precisa passar no health check antes do app iniciar
- ✅ MongoDB precisa responder ao ping antes do app iniciar
- ✅ Elimina race conditions
- ✅ Logs mais limpos, sem erros de conexão

---

## Limite de Registros para Espaço em Disco

### O Problema

O banco completo requer **~170GB** de espaço em disco:
- 140GB para o banco de dados
- 15GB para processamento temporário
- 10GB para arquivos de origem

Muitos ambientes de desenvolvimento/teste não têm esse espaço disponível.

### A Solução Implementada

Novo flag `--max-records` que limita quantos registros são salvos no banco de dados.

#### 1. `transform/venues.go` - Lógica de Limite

**Novo Campo na Struct `venuesTask`:**
```go
type venuesTask struct {
    source     *source
    lookups    *lookups
    kv         kvStorage
    privacy    bool
    dir        string
    db         database
    batchSize  int
    maxRecords int  // NOVO
}
```

**Função `consumeRows()` - Controle de Limite:**
```go
var totalProcessed int

// Durante processamento
if t.maxRecords > 0 && totalProcessed >= t.maxRecords {
    continue // Pula registros após o limite
}

// ... processa registro ...
totalProcessed++

// Verifica se atingiu limite após adicionar
if t.maxRecords > 0 && totalProcessed >= t.maxRecords {
    n, err := t.saveBatch(b)  // Salva batch atual
    // ...
    slog.Info("Reached maximum records limit", "max", t.maxRecords)
    return  // Para o processamento
}
```

**Função `run()` - Progress Bar Ajustada:**
```go
// Usa maxRecords para progress bar se configurado
total := t.source.total
if t.maxRecords > 0 && int64(t.maxRecords) < total {
    total = int64(t.maxRecords)  // Mostra limite, não total
}
bar := progressbar.Default(total)

if t.maxRecords > 0 {
    bar.Describe(fmt.Sprintf("Creating the JSON data for each CNPJ (limited to %d records)", t.maxRecords))
} else {
    bar.Describe("Creating the JSON data for each CNPJ")
}

// ... processamento ...

// Para imediatamente ao atingir limite
if t.maxRecords > 0 && processedRecords >= int64(t.maxRecords) {
    cancel() // Cancela contexto para parar workers
    slog.Info("Reached maximum records limit", "max", t.maxRecords, "processed", processedRecords)
    return nil
}
```

#### 2. `transform/transform.go` - Pipeline Atualizado

**Função `Transform()` - Novo Parâmetro:**
```go
func Transform(dir string, db database, maxDB, maxKV, s int, p bool, maxRecords int) error {
    // ...
    if maxRecords > 0 {
        slog.Info("Maximum records limit enabled", "max", maxRecords)
    }
    if err := createJSONs(dir, pth, db, l, maxDB, s, p, maxRecords); err != nil {
        return err
    }
    // ...
}
```

**Função `createJSONs()` - Propagação do Parâmetro:**
```go
func createJSONs(dir string, pth string, db database, l lookups, maxDB, batchSize int, privacy bool, maxRecords int) error {
    // ...
    j, err := createJSONRecordsTask(dir, db, &l, kv, batchSize, privacy, maxRecords)
    // ...
}
```

**Função `createJSONRecordsTask()` - Inicialização:**
```go
func createJSONRecordsTask(dir string, db database, l *lookups, kv kvStorage, b int, p bool, maxRecords int) (*venuesTask, error) {
    // ...
    t := venuesTask{
        source:     v,
        lookups:    l,
        kv:         kv,
        privacy:    p,
        dir:        dir,
        db:         db,
        batchSize:  b,
        maxRecords: maxRecords,  // NOVO
    }
    return &t, nil
}
```

#### 3. `cmd/transform.go` - Interface CLI

**Nova Variável:**
```go
var (
    maxParallelDBQueries int
    maxParallelKVWrites  int
    batchSize            int
    cleanUp              bool
    noPrivacy            bool
    maxRecords           int  // NOVO
)
```

**Novo Flag CLI:**
```go
transformCmd.Flags().IntVarP(
    &maxRecords, 
    "max-records", 
    "l", 
    0, 
    "maximum number of company records to save (0 = unlimited, useful for testing with limited disk space)"
)
```

**Chamada Atualizada:**
```go
return transform.Transform(dir, db, maxParallelDBQueries, maxParallelKVWrites, batchSize, !noPrivacy, maxRecords)
```

### Uso

```bash
# Sem limite (padrão)
docker-compose run --rm minha-receita transform -d /mnt/data/

# Com limite de 100 mil registros
docker-compose run --rm minha-receita transform -d /mnt/data/ --max-records 100000

# Com limite de 10 mil registros
docker-compose run --rm minha-receita transform -d /mnt/data/ --max-records 10000
```

### Economia de Espaço

| Limite | Tamanho DB | Economia |
|--------|------------|----------|
| 10,000 | ~50 MB | 99.9% |
| 100,000 | ~500 MB | 99.6% |
| 500,000 | ~2.5 GB | 98.2% |
| Ilimitado | ~140 GB | - |

### Comportamento

**O que é limitado:**
- ✅ Registros de empresas (Estabelecimentos)

**O que NÃO é limitado:**
- ❌ Tabelas de lookup (CNAEs, municípios, qualificações, etc.)
- ❌ Índices criados
- ❌ Download dos arquivos

**Progress Bar:**
```
Antes: Creating the JSON data for each CNPJ  0% | (262144/69200268, 10372 it/s)
Depois: Creating the JSON data for each CNPJ (limited to 100000 records)  45% | (45000/100000, 8500 it/s)
```

---

## Documentação Criada

### 1. `README.md` - Seção de Fork Notice

Adicionado no topo do README:
- Aviso claro que é um fork
- Link para projeto original
- Resumo das principais mudanças
- Link para documentação de espaço limitado

### 2. `FORK_CHANGES.md` - Documentação Técnica Completa (Inglês)

Conteúdo:
- Descrição detalhada do problema
- Explicação técnica de cada mudança
- Estrutura WebDAV documentada
- Limitações conhecidas
- Testes realizados
- Referências técnicas

### 3. `docs/limited-space.md` - Guia de Configuração para Disco Limitado (Inglês)

Conteúdo:
- Guia completo de uso do `--max-records`
- Tabelas de economia de espaço
- Exemplos práticos
- Combinações com outros flags
- Troubleshooting
- Recomendações por caso de uso

### 4. `CONTRIBUTING.md` - Atualizado

- Aviso de fork no topo
- Link para projeto original
- Guidelines específicas do fork

### 5. Este documento (`CUSTOMIZAÇÕES.md`)

Resumo completo em português de todas as mudanças.

---

## Resumo Técnico

### Arquivos Modificados

#### Download (Infraestrutura SERPRO+)
- ✅ `download/federal_revenue.go` - Reescrito para WebDAV
- ✅ `download/download.go` - Constantes atualizadas

#### Transform (Arquivos Opcionais)
- ✅ `transform/source.go` - Tax regime files tornados opcionais
- ✅ `transform/venues.go` - Adicionado limite de registros + progress bar
- ✅ `transform/transform.go` - Pipeline atualizado para suportar limite
- ✅ `cmd/transform.go` - Novo flag `--max-records`

#### Docker
- ✅ `docker-compose.yml` - Health checks e depends_on

#### Documentação
- ✅ `README.md` - Fork notice
- ✅ `FORK_CHANGES.md` - Documentação técnica (novo)
- ✅ `CONTRIBUTING.md` - Atualizado
- ✅ `docs/limited-space.md` - Guia de uso (novo)
- ✅ `CUSTOMIZAÇÕES.md` - Este documento (novo)

### Tecnologias Utilizadas

**WebDAV:**
- Protocolo: WebDAV (RFC 4918)
- Método HTTP: PROPFIND
- Autenticação: HTTP Basic Auth
- Parser: encoding/xml (Go standard library)

**Patterns de Design:**
- Optional dependencies (tax regime files)
- Context cancellation (stop on limit)
- Progress tracking with adjusted totals
- Graceful degradation (missing data doesn't break app)

### Compatibilidade

✅ **100% compatível** com API e schema do projeto original  
✅ **Mesma estrutura** de dados no banco  
✅ **Mesmos comandos** CLI (com flags adicionais opcionais)  
✅ **Backward compatible** - flags novos são opcionais

### Limitações Conhecidas

1. **Regime tributário ausente** - Campo `regime_tributario` será nulo
2. **Limite é count simples** - Não filtra por estado/cidade
3. **Download completo ainda necessário** - Limite afeta apenas o banco
4. **Processamento temporário completo** - Badger ainda processa tudo

### Métricas de Sucesso

**Testes Realizados:**
- ✅ URLs listadas corretamente
- ✅ Downloads funcionando com autenticação
- ✅ Arquivos ZIP válidos (verificado com unzip)
- ✅ Transform completa sem tax regime files
- ✅ Docker build bem-sucedido
- ✅ Sem erros de linter
- ✅ Progress bar mostra limite corretamente
- ✅ Processamento para ao atingir limite

**Performance:**
- Download: Funcional (pode ser lento dependendo do SERPRO)
- Transform com limite: ~1 min para 10k registros
- Transform completo: Horas (não mudou)
- Economia de disco: Até 99.9% com limites baixos

---

## Fluxo Completo de Uso

### Para Produção (Dados Completos)
```bash
# 1. Subir bancos de dados
docker-compose up -d postgres mongo

# 2. Download dos dados (~10GB, pode demorar horas)
docker-compose run --rm minha-receita download -d /mnt/data/

# 3. Verificar integridade (opcional)
docker-compose run --rm minha-receita check -d /mnt/data/

# 4. Criar tabelas
docker-compose run --rm minha-receita create

# 5. Transformar (pode demorar horas, gera ~140GB)
docker-compose run --rm minha-receita transform -d /mnt/data/

# 6. Iniciar API
docker-compose up
```

### Para Desenvolvimento com Disco Limitado
```bash
# 1. Subir bancos de dados
docker-compose up -d postgres mongo

# 2. Download dos dados (~10GB)
docker-compose run --rm minha-receita download -d /mnt/data/

# 3. Criar tabelas
docker-compose run --rm minha-receita create

# 4. Transformar com limite (minutos, gera ~500MB)
docker-compose run --rm minha-receita transform -d /mnt/data/ --max-records 100000

# 5. Iniciar API
docker-compose up
```

### Para Testes Rápidos
```bash
# 1. Subir bancos de dados
docker-compose up -d postgres mongo

# 2. Download dos dados (~10GB)
docker-compose run --rm minha-receita download -d /mnt/data/

# 3. Criar dados de amostra (~1 minuto)
docker-compose run --rm minha-receita sample -d /mnt/data/

# 4. Criar tabelas e transformar amostra (~1 minuto, ~50MB)
docker-compose run --rm minha-receita create
docker-compose run --rm minha-receita transform -d data/sample/

# 5. Iniciar API
docker-compose up
```

---

## Manutenção Futura

### Quando a Receita Federal Disponibilizar Regime Tributário

Nenhuma mudança de código será necessária! O código já está preparado:
- Detectará automaticamente os arquivos
- Carregará os dados
- Populará o campo `regime_tributario`

### Atualizações do Token SERPRO+

Se o token público mudar:
```go
// Em download/federal_revenue.go
federalRevenueShareToken = "NOVO_TOKEN_AQUI"
```

### Mudanças na Estrutura de Pastas

Se a estrutura mudar:
```go
// Em download/federal_revenue.go
federalRevenueSourcePath = "/Novo/Caminho/CNPJ"
```

---

## Créditos e Licença

Este fork mantém a mesma licença do projeto original.

**Projeto Original:**
- Autor: [Eduardo Cuducos](https://github.com/cuducos)
- Repositório: https://codeberg.org/cuducos/minha-receita
- Documentação: https://docs.minhareceita.org

**Fork por:** Adaptações para SERPRO+ e recursos para espaço limitado

**Contribua com o projeto original:** https://github.com/sponsors/cuducos

---

## Referências Técnicas

- [RFC 4918 - WebDAV](https://tools.ietf.org/html/rfc4918)
- [Nextcloud WebDAV Documentation](https://docs.nextcloud.com/server/latest/developer_manual/client_apis/WebDAV/)
- [SERPRO+ - Arquivos da Receita Federal](https://arquivos.receitafederal.gov.br/)
- [Dados Abertos da Receita Federal](https://www.gov.br/receitafederal/pt-br/acesso-a-informacao/dados-abertos)
- [Go encoding/xml Package](https://pkg.go.dev/encoding/xml)
- [Context Cancellation in Go](https://go.dev/blog/context)

---

**Última Atualização:** Fevereiro 2026  
**Status:** Testado e funcional  
**Versão Go:** 1.25+  
**Infraestrutura:** SERPRO+ (Nextcloud/WebDAV)
