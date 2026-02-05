# Fork Changes

This document details all changes made to adapt the Minha Receita project to work with the new Brazilian Federal Revenue infrastructure (SERPRO+/Nextcloud).

## Problem

In February 2026, the Brazilian Federal Revenue completely migrated their data infrastructure:
- **Before**: Simple Apache directory listing at `https://arquivos.receitafederal.gov.br/dados/cnpj/dados_abertos_cnpj/`
- **After**: SERPRO+ (Nextcloud) system with WebDAV authentication

The original code was scraping HTML directory listings and using direct file URLs, which no longer work (404 errors).

## Solution Overview

Replaced HTML scraping with WebDAV API calls and updated the download URLs to include authentication credentials.

## Detailed Changes

### 1. `download/federal_revenue.go`

#### New Constants
```go
// Old
federalRevenueURL = "https://arquivos.receitafederal.gov.br/dados/cnpj/"
federalRevenueSourcePath = "dados_abertos_cnpj"

// New
FederalRevenueBaseURL = "https://arquivos.receitafederal.gov.br/public.php/webdav"
federalRevenueShareToken = "gn672Ad4CF8N6TK"
federalRevenueSourcePath = "/Dados/Cadastros/CNPJ"
```

#### New Features
- **WebDAV XML structs** (`webDAVMultistatus`, `webDAVResponse`, etc.) for parsing PROPFIND responses
- **`webDAVList(path)`** - Lists directory contents using WebDAV PROPFIND
- **`federalRevenueGetMostRecentDir()`** - Finds the most recent monthly directory (e.g., "2026-01")
- **`listZipFiles()`** - Lists all .zip files in a directory using WebDAV

#### Modified Functions
- `federalRevenueGetURLs()` - Now uses WebDAV instead of HTML scraping
- `saveUpdatedAt()` - Extracts date from directory name (YYYY-MM format)
- URL generation includes embedded credentials: `https://token:@host/path`

#### Added Import
```go
"encoding/xml" // For parsing WebDAV XML responses
```

### 2. `download/download.go`

#### Changes
- Replaced all references from `federalRevenueURL` to `FederalRevenueBaseURL`
- Updated both `Download()` and `URLs()` functions

### 3. `transform/source.go`

Made tax regime files (Lucro Arbitrado, Lucro Presumido, Lucro Real, Imunes e Isentas) **optional** since they're not available in the new infrastructure.

#### Modified Functions
All these functions now handle empty sources gracefully:
- `pathsForSource()` - Returns warning instead of error for missing tax files
- `newSource()` - Creates empty source when no files found
- `createReaders()` - Skips if no files
- `close()` - Skips if no files
- `resetReaders()` - Skips if no files
- `countLines()` - Returns 0 if no files
- `sendTo()` - Skips if no files

### 4. `docker-compose.yml`

#### Enhanced Service Configuration
```yaml
minha-receita:
  build:
    context: .
    dockerfile: Dockerfile
  depends_on:
    postgres:
      condition: service_healthy
    mongo:
      condition: service_healthy
```

#### Added Health Checks
- PostgreSQL: Uses `pg_isready` command
- MongoDB: Uses `mongosh --eval "db.adminCommand('ping')"`

This ensures the application only starts after databases are fully ready, preventing "database is starting up" errors.

## Technical Details

### WebDAV Authentication
Uses HTTP Basic Authentication with the public share token:
```
Username: gn672Ad4CF8N6TK
Password: (empty)
```

URLs are generated with embedded credentials:
```
https://gn672Ad4CF8N6TK:@arquivos.receitafederal.gov.br/public.php/webdav/path/to/file.zip
```

### Directory Structure
```
/Dados/
├── Cadastros/
│   └── CNPJ/
│       ├── 2023-05/
│       ├── 2023-06/
│       └── 2026-01/  <- Most recent
│           ├── Cnaes.zip
│           ├── Empresas0.zip
│           ├── Empresas1.zip
│           ├── Estabelecimentos0.zip
│           └── ...
└── Obrigacoes_Acessorias/  <- Empty in new infrastructure
```

## Known Limitations

### Missing Tax Regime Data
The following files are currently not available in SERPRO+:
- Lucro Arbitrado.zip
- Lucro Presumido.zip
- Lucro Real.zip
- Imunes e Isentas.zip

**Impact**: The `regime_tributario` field in company records will be empty/null.

**Workaround**: The code now handles this gracefully with warnings instead of errors.

## Testing

All changes were tested and verified:
- ✅ URL listing works (`urls` command)
- ✅ File downloads work with authentication
- ✅ ZIP files are valid and contain expected CSV data
- ✅ Transform process works without tax regime files
- ✅ Docker build succeeds
- ✅ No linter errors

## Compatibility

This fork maintains compatibility with the original project's API and database schema. The only difference is:
- Tax regime data will be absent from records
- Download URLs are different (WebDAV instead of direct HTTP)

## Future Updates

If the Federal Revenue makes tax regime files available in the new infrastructure, no code changes will be needed - the application will automatically detect and process them.

## References

- Original project: https://codeberg.org/cuducos/minha-receita
- SERPRO+ URL: https://arquivos.receitafederal.gov.br/
- WebDAV RFC: https://tools.ietf.org/html/rfc4918
