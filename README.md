<div align="center">

![Minha Receita](docs/minha-receita.svg)

[![Testes](https://ci.codeberg.org/api/badges/15902/status.svg)](https://ci.codeberg.org/repos/15902) [![Apoiadores](https://img.shields.io/github/sponsors/cuducos)](https://github.com/sponsors/cuducos)

API web para consulta de informações do CNPJ (Cadastro Nacional da Pessoa Jurídica) da Receita Federal.

[Documentação](https://docs.minhareceita.org)<br>
[Guia de contribuição](https://docs.minhareceita.org/contributing/)<br>
[Ajude a manter o servidor no ar](https://github.com/sponsors/cuducos)<br>
[Monitor do status do servidor](https://stats.uptimerobot.com/tqpD6AQZqI)<br>
[Métricas do servidor](https://metrics.minhareceita.org)

</div>

---

## 🔄 Fork Notice

This is a fork of the [original Minha Receita project](https://codeberg.org/cuducos/minha-receita) with adjustments for the new Brazilian Federal Revenue infrastructure.

### Key Changes

In February 2026, the Brazilian Federal Revenue migrated their data infrastructure from a simple directory listing to **SERPRO+ (Nextcloud/WebDAV)**. This fork includes:

- **Updated download infrastructure**: Now uses WebDAV API to access Federal Revenue data
- **Automatic authentication**: Embedded credentials for public share access
- **Optional tax regime files**: Tax regime data (Lucro Arbitrado, Lucro Real, etc.) is now optional as it's not yet available in the new infrastructure
- **Health checks in docker-compose**: Ensures databases are ready before starting the application

### Migration Details

**Old infrastructure** (no longer works):
- URL: `https://arquivos.receitafederal.gov.br/dados/cnpj/dados_abertos_cnpj/`
- HTML directory listing with direct file links

**New infrastructure** (SERPRO+/Nextcloud):
- WebDAV endpoint: `https://arquivos.receitafederal.gov.br/public.php/webdav`
- Monthly data folders: `/Dados/Cadastros/CNPJ/YYYY-MM/`
- Requires HTTP Basic Authentication
- Share token: `gn672Ad4CF8N6TK`

### Files Modified

- `download/federal_revenue.go` - WebDAV support for directory listing and file downloads
- `download/download.go` - Updated base URL constants
- `transform/source.go` - Made tax regime files optional
- `docker-compose.yml` - Added health checks and proper service dependencies

For the original upstream project, visit: [https://codeberg.org/cuducos/minha-receita](https://codeberg.org/cuducos/minha-receita)

**📦 Limited Disk Space?** See [docs/limited-space.md](docs/limited-space.md) for configuration options.

**🇧🇷 Documentação em Português:** [CUSTOMIZAÇÕES.md](CUSTOMIZAÇÕES.md) - Resumo completo de todas as customizações

---
