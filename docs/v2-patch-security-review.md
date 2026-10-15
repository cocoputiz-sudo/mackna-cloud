# MackNa v2.0 — Auditoria do patch recebido (15/10/2026)

**Origem:** arquivo fornecido pelo usuário `mackna-cloud-v2.0.patch` (commit 520a814b, baseado em PR #4 / 8ad58a8). Mudanças propostas: 51 arquivos, ~13.9 mil linhas novas. **Não equivale a release validado.**

## Resultado da revisão inicial

| Área | O que o patch propõe | Decisão |
| --- | --- | --- |
| Deploys GitHub (Dockerfile/Node/Python/Go/estático) | Clone de repositório + `docker build` + imagens locais | Manter fora da BTV11 até existir sandbox independente, rate limiting e política de rede |
| PostgreSQL/Redis templates | Criação de volumes, bancos e integração ao Manager | Prototipar no Node 02 descartável; não executar DDL nem `pg_restore` na IMORTAIS |
| Cofre | AES-256-GCM e chave 0600 | Adotar a ideia, com segurança na criação concorrente e testes de adulteração (`cloudsecrets`) |
| Proxy Caddy/HTTPS | Proxy multi-projeto, domínios e certificados automáticos | Requer testes de roteamento, redes existentes, wildcard DNS, ACME e rollback |
| Workers TLS | TLS1.3 e pin de certificado para comunicação entre máquinas | Exige teste de rotação do pin, expiração e revogação, firewall privado e falhas parciais |
| Auditoria | Eventos administrativos com hash do SQL | Preservar auditoria atual; nunca guardar texto SQL, segredos ou tokens |
| Instalação | `scripts/mackna-install.sh` modifica host e inicia Agent/Gate/Deployer/Proxy | **Não executar diretamente** até separar verificação, dry-run e operações transacionais |
| Rollback e réplica | Rolling deploy/rollback e reconciliação | Só homologar com falhas simuladas, saúde e persistência em contêineres descartáveis |

## Bloqueadores concretos para aceitar a v2

1. **Proteção do host tarde demais.** No instalador, a inspeção de contêineres IMORTAIS acontece **depois** de passos que podem modificar `apt` e Docker. Mover preflight para o começo do instalador e recusar host não vazio antes de qualquer mutação.
2. **Rollback de sistema incompleto.** Restaurar binários/systemd não reverte automaticamente `apt`, regras UFW, redes Docker, imagens, arquivos e serviços persistentes criados anteriormente. Precisa plano separado de rollback para infraestrutura vs release do Agent.
3. **Verificação de propriedade de rede Docker.** `internal/docker.Client.NetworkEnsure` devolve uma rede encontrada pelo nome sem garantir `mackna.managed=true` ou opções/subnet esperadas. Nunca se conectar a rede preexistente de outro sistema apenas por igualdade do nome.
4. **Segredos no build.** `internal/cloud.Builder.Build` passa `BuildEnv` por `--build-arg`. Mesmo sem pôr o valor literal na linha de comando, build args podem aparecer em histórico/cache/logs e estar acessíveis ao Dockerfile. Usar BuildKit `--secret` com escopo mínimo, nunca tratar `--build-arg` como cofre.
5. **Execução de código de terceiros.** Clonar e construir Dockerfiles arbitrários roda comandos de terceiros no executor Docker com rede e recursos do host. Isolar builders e proibir compartilhamento do Docker socket principal do controle.
6. **Proxy faz ponte entre redes.** O Caddy é conectado a redes de vários projetos; confirmar desconexão ao remover serviço e testar isolamento real/rotas, especialmente depois de crashes e reconciliadores.
7. **Fail-closed no PostgreSQL e restore.** Os novos endpoints administrativos devem ser estritamente opt-in, limitados a bancos próprios e testados com restore obrigatório para banco novo, manifesto SHA-256 válido e limites de disco.
8. **Webhooks e rotas públicas.** Exigir assinatura HMAC, tolerância de replay, limites de tamanho e origem, autenticação separada para painel e logs; simular HTTPS atrás do Caddy sem publicar o painel de controle até concluir os testes.
9. **Regras de domínio.** O uso de sslip.io/IP público é conveniente para laboratório, mas **não é uma estratégia de DNS ou certificação garantida para produção**.
10. **Homologação CI não substitui servidor real.** Um workflow com Docker descartável pode demonstrar funcionalidade, não segurança 24/7, disponibilidade garantida, backups restauráveis ou zero downtime.

## Implementações iniciais deste PR

- `backend/internal/cloudsecrets`: base de AES-256-GCM autenticada, prefixo versionado, nonce aleatório, chave exclusivamente criada com permissões 0600 e testes de corrupção/adulteração/concorrrência.
- `scripts/v2-node02-admission.sh --check`: **somente leitura**, recusa host com serviços Docker preexistentes, SO incompatível, falta de espaço/RAM ou portas ocupadas antes de qualquer instalador.
- Esta auditoria serve como critérios de aceite e lista de funções que **não serão ativadas na BTV11**.

## Próxima ordem de execução

1. Validar arquitetura/CPU/RAM/SSD do all-in-one e rodar preflight read-only, sem instalar nada no servidor.
2. Criar ambiente `node02-lab` com Docker/volumes de testes, isolado do Railway e IMORTAIS.
3. Integrar e testar separadamente catálogo de projetos, agendador, builder sandbox, roteador Caddy, DB templates e auditoria.
4. Simular falhas (build cancelado, rede perdida, contêiner morto, falta de espaço, certificado, rollback, restore adulterado).
5. Somente após um release com CI e testes de campo verdes, ativar instalação administrativa de v2 no Node 02. Nunca no Node 01 compartilhado.

**O patch v2 recebido foi usado como referência e fonte para revisão; não foi aplicado integralmente nem executado.**
