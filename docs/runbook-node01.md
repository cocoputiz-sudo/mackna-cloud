# MackNa Cloud PR #2 — Node 01 (Debian 13 ARM64)

**Status: código de homologação, não instalado automaticamente na BTV11.** Não há comandos de escrita na API; a IMORTAIS e o Railway original continuam intocados.

## Arquitetura conjunta

- `mackna-agent` (também chamado `macknad` no desenho do Claude): control plane Go, dashboard HTML, tokens `read`, `logs`, `admin`, API HTTP **somente loopback** `127.0.0.1:8090`.
- `mackna-gate`: processo separado, única peça MackNa no grupo `docker`; acessa `/var/run/docker.sock` e oferece um socket Unix intermediário controlado.
- O agente **não pertence ao grupo docker**; o gate normaliza o JSON do Docker e remove `Config.Env`, comandos, mounts e labels arbitrárias antes de retorná-lo.
- O dashboard do meu PR #1 foi mantido e expandido com temperatura. O gate e o armazenamento de tokens são adaptações das ideias fornecidas pelo Claude, não cópias literais do patch externo.

## Requisitos

- Debian 13 ARM64 na BTV11, Docker instalado e funcionando.
- Contêineres IMORTAIS `mackna-btv-app-1` / `mackna-btv-db-1` existentes e independentes.
- Usuário root disponível via `su -`, sem depender de `sudo`.
- O CI deste PR aprovado.

## Download

Em `Actions > MackNa Cloud PR2 - Node 01 > Run aprovado > Artifacts`, baixar o pacote `mackna-pr2-node01-arm64`. Extrair no PC ou no servidor em uma pasta temporária.

O pacote contém `dist/mackna-agent-linux-arm64`, `dist/mackna-gate-linux-arm64`, `deploy/systemd/*.service` e `scripts/install-node.sh`.

**Não usar o artefato antigo do PR #1**, pois ele usa HTTP Basic com outro contrato e executava diretamente consultas à CLI Docker.

## Instalação manual

Com os arquivos disponíveis na BTV11, dentro da pasta extraída:

```bash
su -
bash scripts/install-node.sh dist/mackna-agent-linux-arm64 dist/mackna-gate-linux-arm64
systemctl --no-pager status mackna-gate mackna-agent
curl -i http://127.0.0.1:8090/api/v1/health
```

Nenhuma etapa reinicia o Docker, o PostgreSQL ou o backend IMORTAIS.

Criar o primeiro token **localmente**, como usuário `mackna`:

```bash
su -s /bin/sh mackna -c '/usr/local/bin/mackna-agent token create administrador admin'
```

O token tem prefixo `mkn_` e aparece **uma única vez**, no terminal local; não o envie pelo Discord, GitHub ou chat.

## Acesso pelo navegador

No PowerShell do Windows, criar túnel SSH (mantê-lo aberto):

```powershell
ssh -N -L 8090:127.0.0.1:8090 imortais@192.168.15.11
```

Abra `http://127.0.0.1:8090/` e autentique usando:

- Usuário: `admin`;
- Senha: **seu token `mkn_...`**.

O endpoint `/api/v1/health` não exige token, os demais exigem HTTP Bearer (ou Basic somente como login do dashboard). Exemplo de cliente CLI:

```bash
curl -H "Authorization: Bearer SEU_TOKEN" http://127.0.0.1:8090/api/v1/system
```

## API

| Rota | Escopo | Finalidade |
| --- | --- | --- |
| `GET /api/v1/health` | Público, localhost | Saúde do processo e conexão com gate/Docker |
| `GET /` | `read` via Basic | Dashboard HTML com host e contêineres |
| `GET /api/v1/system`, `GET /api/v1/node` | `read` | Host: arquitetura, memória, disco, uptime, carga e temperatura |
| `GET /api/v1/containers` | `read` | Lista sanitizada |
| `GET /api/v1/projects` | `read` | Contêineres agrupados pelo projeto Docker Compose |
| `GET /api/v1/containers/{id}` | `read` | Inspect filtrado, sem dados sensíveis |
| `GET /api/v1/containers/{id}/stats` | `read` | CPU e RAM da aplicação |
| `GET /api/v1/containers/{id}/logs?tail=200` | `logs` | Logs, máximo 1000 linhas e 1MiB no gate |

Todos os métodos `POST`, `PATCH`, `PUT` e `DELETE` da API são rejeitados. O gate rejeita endpoints Docker de criação, restart, exec, export, archive, etc. A API não expõe o Docker Socket.

## Segurança e pendências de PR #2

- O gate roda no grupo `docker`, **efetivamente privilegiado**, mesmo com interface GET. Código do gate precisa receber auditoria antes de uso em produção.
- O gate filtra inspect/list, mas **logs de aplicativos podem conter segredos**, acessíveis somente com escopo `logs`. Não compartilhar tokens com esse escopo sem necessidade.
- Os tokens são segredos de alta entropia; apenas SHA-256 fica no arquivo JSON `0600`, sem token em plaintext no repositório. Revogação via CLI.
- As auditorias desta versão são linhas JSON nos logs do `journald`, sem histórico próprio pesquisável via API.
- Ainda não existem login com cookies, sessão, UI de criar projetos, deploys, rollback, backups, multi-node ou acesso remoto HTTPS.
- `read` pode acessar inventário e métricas, `logs` pode ver logs, `admin` herda todos os escopos. **Não conceder `admin` a agentes sem necessidade**.
- A interface HTTP é disponibilizada somente em loopback. O túnel SSH protege o tráfego.
- Medir memória do gate e agente por 30 minutos, a saúde da IMORTAIS e estabilidade por 24 horas antes de homologar.

## Rollback

```bash
systemctl disable --now mackna-agent mackna-gate
```

Isso desativa apenas os componentes novos da MackNa, sem parar Docker ou os contêineres IMORTAIS.

**Não executar mudanças em `/etc/docker/daemon.json` nem reiniciar Docker como parte da instalação do PR #2.**


## PR #3 (v0.3.0) — console visual de projetos e serviços

Este PR é incremental e parte do commit 240cb841 do PR #2. Enquanto o PR #2 não for mesclado, o PR #3 tem base na branch do PR #2. Mesclar na ordem #2 e depois #3, revisando a base e o CI; não implantar automaticamente.

O painel original em / continua disponível. A rota /projects permite navegar pelos projetos detectados pelas labels permitidas do Docker Compose; /projects/view?name=... mostra os serviços; /services/view?id=... mostra estado, saúde, imagem, reinicializações, CPU e memória. O painel não exige JavaScript e funciona com HTTP Basic sobre o túnel SSH já configurado.

Logs são consultados apenas sob solicitação explícita na página do serviço. É necessário token com escopo logs ou admin, além do escopo read; o token read sozinho recebe HTTP 403. O limite é de 1 a 1000 linhas, com 1 MiB máximo na resposta do gate. As linhas são escapadas como texto HTML, sem interpretação como script; logs podem conter segredos e não devem ser compartilhados.

Não há exec, restart, deploy, alteração de Compose, mudança de imagem nem manipulação do Docker Socket pelo controlador. Toda consulta aos serviços utiliza a lista normalizada pelo gate. Falhas individuais de inspect/stats/logs não interrompem o painel. O pacote de CI desta etapa chama-se mackna-pr3-node01-arm64.

Para homologar, primeiro instalar somente quando autorizado e depois verificar: navegação do projeto IMORTAIS, dois serviços healthy, métricas e logs com token adequado; logs negados com token read; saúde do backend e do PostgreSQL antes e depois; temperatura, RAM e espaço livres estáveis. Se a UI apresentar falhas, reverter exclusivamente os binários MackNa, preservando Docker e contêineres. A atualização do gate não altera a allowlist nem habilita escrita.
