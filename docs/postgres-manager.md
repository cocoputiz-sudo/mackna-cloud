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
