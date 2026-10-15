# Auditoria técnica: `mackna-cloud-v1.0.patch` (15/10/2026)

## Proveniência e escopo
Arquivo recebido do usuário com dois commits em formato `git format-patch`, 23 arquivos no primeiro commit e três no segundo, adicionando aproximadamente 4.000 linhas ao MackNa Cloud. O código foi avaliado **como proposta**, não aplicado integralmente. Não há evidência de que tenha sido homologado em uma BTV11 ou em um nó maior.

## Aproveitado no PR incremental
1. Monitoramento com consultas PostgreSQL predefinidas, **somente no banco descartável `mackna_lab`**, preservando `BEGIN READ ONLY`, `mackna_reader` e o endpoint existente.
2. Estatísticas úteis de conexões, tabelas, índices, locks agrupados, parâmetros seguros, estatísticas do banco e extensões. Não devolver `pg_stat_activity.query`, hashes/valores SQL, passwords, conn strings ou PID de outra sessão.
3. Trilhas locais limitadas e rotativas de acesso, token ID (nunca segredo), path **sem query string**, status e timestamp. Acesso ao endpoint de auditoria restrito a token `admin`.
4. Testes de RBAC, UI CSP, allowlist e integração PostgreSQL 16.

## Bloqueadores antes de v1 administrativa

| Prioridade | Achado observado no patch | Consequência / tratamento |
| --- | --- | --- |
| **Crítica** | `scripts/mackna-v1-install.sh` substitui e reinicia `mackna-gate`, altera systemd e instala dependências via `apt-get`; pode adicionar repositório PGDG | Viola o requisito de não tocar Gate/IMORTAIS no nó existente; exigirá **instalador para nó novo** separado, com aprovação e testes de rollback |
| **Crítica** | Instalador cria `mackna_admin` e `postgres.json` com `allow_write:true`; altera papéis/senhas do laboratório | A mudança não é somente-leitura e o rollback do binário não desfaz permissões ou alterações do PostgreSQL; **desabilitado** |
| **Crítica** | `RestoreToNewDatabase` aceita arquivo de dump se o manifesto estiver ausente, inválido ou não contiver SHA | Não é fail-closed: backup adulterado pode ser aceito; exigir manifesto obrigatório, validar schema/instância/versão/bytes/hash e testar restauração isolada |
| **Alta** | Auditoria guarda `audit.RedactSQL(body.SQL)` com regex e mantém até 300 caracteres de SQL | Pode revelar dados pessoais e literais fora de `PASSWORD`; registrar **somente hash e categoria**, sem SQL bruto |
| **Alta** | Editor SQL aceita comandos enviados pelo navegador; `READ ONLY` não elimina riscos de funções com efeitos colaterais | Exige sandbox isolada, role sem `SECURITY DEFINER` perigoso, quotas e testes negativos |
| **Alta** | `Manager.refresh` pode fechar um pool enquanto uma consulta mantém referência; `AdminConn` administra bancos e roles globalmente | Requer gestão de concorrência/refcounts e privilégios restritos por cluster, além de instâncias protegidas por padrão |
| **Alta** | `pg_restore` pode durar 60 min, mas o servidor define `WriteTimeout:2m` | Requisições longas falharão/informarão estado ambíguo; mover operações longas para jobs assíncronos auditados |
| **Média** | A gestão de backups usa arquivos no host/eMMC com retenção; não há cópia externa confirmada | Reservar espaço, quotas/alertas, retenção e confirmação de restore testado antes de chamar um backup de seguro |
| **Média** | `README` do patch chama o produto de "v1.0 completa" | O próprio patch deixa deployments, HTTPS, multino e integração IA de fora; status deve ser funcionalidade por funcionalidade |

## Estratégia para uma instância maior
A evolução para outro servidor exige definir **CPU, RAM, disco, SO, conectividade e objetivo**. Até lá:
- Novas funcionalidades devem operar somente no laboratório existente, ser `GET` autenticado e não tocar a IMORTAIS.
- Projeto de hospedagem: GitHub Actions/GHCR, plano de implantação por serviço com nomes reservados, quotas Docker/Compose, healthcheck e rollback, sem permitir `docker exec` arbitrário.
- Backups: jobs assíncronos, manifesto verificado, criptografia/destino externo e restore apenas em banco novo sem elevação de privilégios.
- Multi-node: agentes com autenticação mútua e privilégios mínimos, sem disponibilizar SSH nem socket Docker diretamente pela API.
- SQL administrativo deve ser separado, aprovado e explicitamente opt-in; nunca ativado como efeito colateral de instalar o painel.
- Antes de liberar em nova instância: testes de resistência de autenticação, tempo, interrupção, rollback e limites de recursos; evidências de isolamento dos serviços existentes.

## Estado de implantação
Nenhum script do patch original foi executado no servidor. O PR incremental não cria/reinicia Docker, Gate, volumes, IMORTAIS, papéis de banco ou rotas de escrita. As mudanças estão apenas no código até revisão, CI e homologação.
