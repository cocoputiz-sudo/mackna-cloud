# Arquitetura inicial — MackNa Cloud

## Estado inicial

- **Node 01**: TV Box BTV11 S905X3, Debian 13 ARM64 na eMMC (~14,7 GiB), 1,8 GiB RAM.
- **Workload de homologação já existente**: `mackna-btv-db-1` (PostgreSQL 16) e `mackna-btv-app-1` (IMORTAIS backend, sem login Discord).
- **Plano de controle MackNa**: código Go desta branch. Ainda não implantado na BTV11.

## MVP

O agente é um serviço systemd com escuta fixa em `127.0.0.1:8090`. Autenticação HTTP Basic sobre **túnel SSH**. Credencial gerada no host e carregada por um EnvironmentFile inacessível a usuários comuns.

Rotas:

| Rota | Autenticação | Uso |
| --- | --- | --- |
| GET `/api/v1/health` | Pública no loopback | Liveness da API |
| GET `/api/v1/system` | Basic | Memória, disco, uptime, carga, arquitetura |
| GET `/api/v1/containers` | Basic | Inventário somente leitura via `docker ps -a --format '{{json .}}'` |
| GET `/` | Basic | Dashboard HTML simples, auto-refresh sem JavaScript |

Sem rotas de escrita, sem execução de comandos arbitrários, sem Socket Docker oferecido aos clientes HTTP e sem credenciais no repositório.

## Segurança e limites

O agente necessita do grupo `docker` para consultar o daemon. Membros desse grupo têm privilégios equivalentes a root: tratar o processo como sensível e não expor sua porta. Mais adiante, separar consulta via proxy com allowlist e montar um plano de controle não privilegiado.

O MVP não cria deploys, não provisiona bancos, não tem multi-tenancy, nem implementa backups. `health` indica que o processo web está vivo, não prova disponibilidade do Docker.

Não confundir serviço instalado localmente com distribuição de produção validada. Exigir testes, revisão e implantação manual inicial.

## Próximas entregas

1. Métricas históricas e logs com limites, sem expor segredos.
2. Plano de controle e agente desacoplados, autenticação de usuários.
3. Operações mutáveis sob autorização, validação de alvos, auditoria, rollback.
4. Build por Actions e deploy controlado via GHCR.
5. Persistência, backups restauráveis, domínios e múltiplos nós.
