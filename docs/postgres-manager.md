# PostgreSQL Manager — MackNa Cloud

## Objetivo

Console de administração PostgreSQL integrado à MackNa Cloud, inspirado no fluxo do Railway, sem copiar interface proprietária. Implementação incremental para Debian 13 ARM64, BTV11 (~1,8 GiB RAM), PostgreSQL 16 e IMORTAIS em homologação.

## Arquitetura

- **PostgreSQL Gate separado** do Docker Gate, com usuário Unix dedicado e conexão PostgreSQL TCP/Unix interna. Não conceder ao processo web credenciais de superusuário nem acesso ao Docker Socket.
- **Control plane Go** renderiza as páginas e valida sessões, RBAC, CSRF, quotas e auditoria. Proibir SQL interpolado; valores por parâmetros e identificadores por allowlist/quoting seguro.
- **Credenciais** fora do Git, nunca em URLs, logs, páginas ou responses; configuração por arquivo root-only/secret mount. TLS obrigatório em acesso remoto. Não importar segredos do Railway.
- **Conexões** limitadas por pool, timeout e cancelamento de consulta; não executar SQL arbitrário a partir de um token genérico de leitura.
- **Nó**: a BTV11 executa consultas leves e paginação; backups e exports têm limites de espaço, memória, CPU e concorrência.

## Entregas em fases

### A. Inventário e observabilidade (somente leitura)

- Listar instâncias PostgreSQL explicitamente cadastradas, com health, versão, uptime, tamanho, conexões, cache hit, sessões, locks e estatísticas de tabelas.
- Schemas, tabelas, views, materialized views, colunas, tipos, PK/FK, índices, constraints e estimativas de linhas via pg_catalog/information_schema.
- Navegador de dados paginado com limite e ordenação segura, sem SELECT * ilimitado. Dados sensíveis mascarados por padrão em perfis não administrativos.
- Páginas: Overview, Data, Query, Metrics, Backups, Users, Settings, Audit. Rotas inexistentes retornam 404/403, não simulam dados.

### B. Console SQL controlado

- Editor SQL com resultado tabular, duração, quantidade de linhas, paginação, export CSV com limite e cancelamento.
- Role db_read executa apenas consulta read-only dentro de transação READ ONLY e com statement_timeout. Impedir múltiplas instruções e comandos que escrevam, inclusive via funções perigosas; usar credencial PostgreSQL efetivamente read-only.
- Role db_write em conexão separada com privilégios mínimos; confirmação explícita, preview do impacto quando possível, transação, limite de tempo e auditoria.
- Comandos de manutenção/administração com privilégio próprio; nunca conceder superuser via UI.

### C. Gestão e recuperação

- Criação de database/schema/role com formulários tipados, grants explícitos e RBAC; sem exibir senhas após criação.
- Backup pg_dump custom e restauração pg_restore com manifesto, checksum, retenção, armazenamento externo e testes reais de restore em banco descartável. Backups automáticos apenas após validar armazenamento; não armazenar o único backup na eMMC.
- Restore, DROP, TRUNCATE e operações destrutivas exigem reautenticação, confirmação contextual, snapshot e plano de reversão; nunca executar automaticamente.
- Métricas históricas com retenção curta e coleta controlada; pg_stat_statements apenas se habilitado e suportado, sem alterar postgresql.conf automaticamente.

## Contratos de API propostos (não implementados nesta fundação)

- GET /api/v1/postgres/instances
- GET /api/v1/postgres/instances/{id}/overview
- GET /api/v1/postgres/instances/{id}/schemas
- GET /api/v1/postgres/instances/{id}/tables?schema=public&limit=50&cursor=...
- GET /api/v1/postgres/instances/{id}/tables/{table}/rows?limit=50&cursor=...
- GET /api/v1/postgres/instances/{id}/metrics
- POST /api/v1/postgres/instances/{id}/query (separado por RBAC e CSRF)
- POST /api/v1/postgres/instances/{id}/backups (autorização administrativa)
- POST /api/v1/postgres/instances/{id}/restore (autorização reforçada)

Nenhuma rota acima está ativa nesta fase; o contrato é apenas planejamento. No backend atual, o bloqueio global de métodos diferentes de GET permanece intacto.

## Critérios de aceite

1. Isolamento comprovado: token read da MackNa não permite SQL nem acessar logs/segredos.
2. Sem alteração no PostgreSQL 16 da IMORTAIS antes da homologação em banco descartável.
3. Testes de SQL injection, CSRF, escalonamento de privilégio, timeout, cancelamento, paginação, dados sensíveis e concorrência.
4. Testes reais de backup/restore com integridade e checksum; falha de backup nunca reportada como sucesso.
5. Sem interrupção do Docker, backend IMORTAIS, PostgreSQL ou Railway.
6. Sem build pesado na BTV11; CI ARM64 externo.

## Escopo deste PR

**Somente arcabouço de produto, documentação de segurança e contrato de implementação.** Nenhuma operação de banco foi ativada. A primeira entrega executável deverá ser a fase A, em PR subsequente, após aprovação da arquitetura e dos acessos.

## Requisitos vinculantes de homologação (14/10/2026)

**Bloqueio de avanço:** antes de implementar a Fase A, instalar e homologar PR #3 na BTV11, com evidências de `systemctl status mackna-agent`, `GET /api/v1/node`, `GET /api/v1/projects` e `free -m`; somente após aprovação fazer merge do PR #3 na main. Este PR #4 permanece draft e só poderá ser rebaseado sobre main depois do merge do PR #3. Não inferir homologação de CI.

### Isolamento de credenciais e transações

- Driver obrigatório: `github.com/jackc/pgx/v5`, Go puro.
- Acesso de leitura com `mackna_reader`: `GRANT pg_read_all_data TO mackna_reader`; `ALTER ROLE mackna_reader SET default_transaction_read_only = on`, `statement_timeout = '15s'`, `work_mem = '4MB'`, `idle_in_transaction_session_timeout = '30s'`. `pg_read_all_data` abrange todos os dados: esta role deve existir **somente em instância de laboratório inicialmente** e seu uso em outras instâncias requer aprovação e isolamento adicionais.
- Cada execução do console usa `BEGIN READ ONLY`, protocolo estendido pgx (uma instrução por execução), com credencial sem privilégios de escrita; parser SQL é apenas aviso UX, jamais barreira de segurança.
- Testar funções com efeitos colaterais; `READ ONLY` não substitui revogação de `EXECUTE` de funções perigosas, inclusive funções `SECURITY DEFINER`. O banco de laboratório não deve ter funções perigosas disponíveis para o leitor.
- Administração isolada na role `mackna_admin` com `CREATEDB` e `CREATEROLE`, **sem SUPERUSER**. Não usar superusuário em nenhum processo da MackNa. Provisionamento inicial privilegiado é ação externa do operador.
- PostgreSQL somente em `127.0.0.1:5432` no Compose ou socket Unix compartilhado; nunca `0.0.0.0`. Atenção: containers distintos não compartilham loopback; escolher rede/socket compatível sem exposição pública.

### Limites de recursos na BTV11

- Respostas: máximo **1.000 linhas OU 5 MiB**, o que ocorrer primeiro; paginação por chave única estável (keyset), não OFFSET ilimitado. Tabelas sem chave única ficam sem navegação de linhas até haver estratégia segura.
- Uma consulta simultânea por usuário; pool máximo de três conexões por instância, incluindo métricas. Aplicar limite de memória e timeout na camada HTTP.
- Cancelamento real via `pg_cancel_backend(pid)` com PID pertencente à sessão do próprio usuário, verificando ownership no servidor; o cancelamento requer conexão de controle e permissões adequadas, que devem ser explicitamente validadas sem conceder `pg_signal_backend` indiscriminadamente. Preferir cancelamento do contexto pgx com confirmação de interrupção.

### Backup, restore e auditoria (fases posteriores, não ativar na Fase A)

- `pg_dump` e `pg_restore` versão 16, iguais ao PostgreSQL 16 do laboratório, executados em contêiner efêmero `postgres:16-alpine` pela operação nomeada `pg_backup` no Docker Gate, **sem `exec` genérico**. Verificar compatibilidade de versão para cada instância cadastrada antes de executar.
- Backup para download ou destino externo; na eMMC manter **no máximo um** arquivo temporário, com limpeza após transferência, retenção e checksum.
- Restaurar exclusivamente em novo banco `<db>_restore_<timestamp>`; validar tabelas e contagens, então exigir confirmação humana antes de qualquer troca de nomes. Proibido restaurar diretamente no banco ativo. Planejar conexões abertas, rollback e downtime da troca.
- Auditoria: usuário, tipo de comando, SHA-256 do texto SQL, duração e linhas afetadas/retornadas; **nunca armazenar SQL em texto**, nem credenciais/PASSWORD. Mascarar parâmetros e mensagens de erro potencialmente sensíveis.

### Primeiro PR executável: somente Fase A

- Instâncias explicitamente cadastradas; overview: versão, uptime, tamanho, conexões, cache hit.
- Schemas, tabelas, colunas, PK/FK e índices via catálogos; navegador de linhas com paginação por chave.
- Tudo executado exclusivamente como `mackna_reader`, sem CRUD, editor SQL arbitrário, backups, restore, administração de roles ou acesso ao banco IMORTAIS.
- GitHub Actions: testes de integração com service container `postgres:16-alpine` e banco descartável `mackna_lab`. Provas negativas obrigatórias: `INSERT`, `UPDATE`, `DELETE`, `CREATE` e função com efeito colateral falham com a credencial reader. Verificar também paginação, limites, cancelamento, conexões e ausência de exposição de segredos.
- Não modificar, conectar nem migrar a instância PostgreSQL da IMORTAIS nesta fase.
