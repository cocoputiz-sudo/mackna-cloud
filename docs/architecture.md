# Arquitetura MackNa Cloud — PR #2 (14/10/2026)

## Infraestrutura confirmada
Node 01: BTV11, S905X3 ARM64, Debian 13 na eMMC Biwin de ~14,7GiB, ~1,8GiB RAM, Docker Engine 29 e Compose.
A aplicação IMORTAIS de preview e PostgreSQL já rodam em contêineres independentes. Não alterá-los.

## Dois serviços separados

```text
Navegador no Windows
   | túnel SSH 127.0.0.1:8090
   v
mackna-agent / macknad (usuário mackna, fora do grupo docker)
   | Bearer para API; Basic token somente para dashboard
   | leitura de /proc, /sys e filesystem
   | Unix socket /run/mackna-gate/docker-ro.sock
   v
mackna-gate (usuário mackna-gate, grupo docker)
   | allowlist de GET, argumentos fixados, timeouts e limites
   | sanitização obrigatória de inspect e list
   v
/var/run/docker.sock  ---> Docker Engine ---> contêineres
```

O socket do Docker é uma interface com privilégios equivalentes a root, independentemente de ser utilizado para GET no código da MackNa. Por isso o `mackna-agent` não recebe permissão de acesso direto e nenhum socket é exposto por TCP. Apenas o gate participa do grupo `docker`.

## Partes aproveitadas

- **PR #1 (ChatGPT)**: dashboard HTML embutido no Go, abordagem mínima de consumo de memória, monitoramento e build ARM64, acesso HTTP em loopback.
- **Patch fornecido pelo Claude**: arquitetura split `macknad` + `mackna-gate`, tokens por escopo, Unix socket, política allowlist, estatísticas e logs limitados.
- **Correções aplicadas na integração**: o gate filtra informações sensíveis em suas próprias respostas Docker. Em particular, inspect não retorna `Config.Env`, `Cmd`, `Entrypoint`, `Mounts.Source` ou labels arbitrárias. As flags de follow/stream em logs são rejeitadas. A documentação e instalador não exigem `sudo` nem reiniciam Docker.

Esta implementação não é uma aplicação textual literal do patch Claude, que não estava em branch do GitHub. É uma versão integrada desenvolvida e testada separadamente.

## Autenticação e auditoria

A API requer token de 256 bits com hash SHA-256 armazenado em arquivo `0600`. Escopos:
- `read`: host e contêineres, incluindo inspeção sanitizada e estatísticas;
- `logs`: acesso adicional à saída dos aplicativos, que pode conter segredos;
- `admin`: permissões anteriores e administração prevista no plano futuro.

A revogação é realizada por CLI, com atualização do JSON por rename atômico. A auditoria inicial escreve eventos JSON com ID de token (nunca o segredo) no journald, que tem sua própria política de retenção. Auditoria persistente pesquisável e rotação própria ainda não foram implementadas.

## API PR2

- `GET /api/v1/health`: health do Docker via gate, 503 quando indisponível;
- `GET /`: HTML do dashboard, login Basic com token `mkn_` como senha;
- `GET /api/v1/system`, `/api/v1/node`: arquitetura, RAM, disco, uptime, temperatura e load;
- `GET /api/v1/containers`: lista sanitizada;
- `GET /api/v1/projects`: agrupamento por label Compose;
- `GET /api/v1/containers/{id}`: detalhes sem segredos;
- `GET /api/v1/containers/{id}/stats`: CPU e memória com streaming desligado;
- `GET /api/v1/containers/{id}/logs?tail=N`: limite 1000 linhas e 1MiB, somente escopo logs.

## Limites de segurança

O gate possui acesso root-equivalente a Docker e seu Unix socket é alcançável pelo serviço do controlador. Uma vulnerabilidade no agente ainda pode permitir leitura de dados autorizados pelo gate (especialmente logs). A separação reduz exposição de operações destrutivas, **não equivale a isolamento absoluto**.

Os métodos POST, DELETE, PUT e PATCH não são aceitos. Nunca fornecer shell ou Docker Socket direto a agentes de IA. O serviço só escuta `127.0.0.1:8090`, requer túnel SSH e não deve ser publicado na internet. Não há interface de deploy, ações de reiniciar, rollback, backup ou suporte multi-tenant.

## Próximas fases

1. Homologação PR2 (CI, portas, autenticação, um dia de funcionamento, CPU/RAM real, taxa de escrita eMMC).
2. Histórico de métricas leve, proteção contra excesso de tentativas, auditoria rotativa e UI autenticada com sessão.
3. Deploy GHCR com projetos gerenciados explicitamente, digest fixo, revisão prévia e rollback.
4. PostgreSQL gerenciado, backups com restauração testada.
5. Domínios, HTTPS e múltiplos nós com mTLS.
6. Conector/MCP para agentes com leitura limitada e ações sujeitas a aprovação.
