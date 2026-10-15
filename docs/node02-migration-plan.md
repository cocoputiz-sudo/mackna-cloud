# MackNa Cloud: laboratório BTV11 → novo Node 02 (all-in-one)

**Plano de migração. Não instalar nem alterar nada automaticamente.** Os testes atuais na BTV11 são uma prova funcional, não um dimensionamento de produção.

## Papéis dos dois equipamentos

| Nó | Arquitetura | Capacidade conhecida | Papel |
| --- | --- | --- | --- |
| Node 01 (BTV11) | Linux ARM64/aarch64 | ~1,8 GiB RAM; ~13 GiB raiz; Amlogic S905X3 | Laboratório: validar autenticação, painel, API, limites, rollback e integração com um PostgreSQL descartável |
| Node 02 (all-in-one) | **a confirmar**; provável Linux amd64/x86_64 | alvo **8 GiB RAM DDR3, Intel Core i3-2100 (2 núcleos físicos / 4 threads, 3,1 GHz), SSD 120 GB**, segundo inspeção do usuário; placa-mãe, slots e SO a confirmar | Servidor principal candidato: hospedar MackNa e serviços isolados após homologação |

**Não inferir automaticamente que o novo equipamento é AMD64 ou já tem Linux.**
O workflow `.github/workflows/node02-amd64.yml` produz **binários candidatos** de Agent + Gate para AMD64, sem instalar nada. O workflow ARM64 da BTV11 continua separado. O agente e o Gate são binários diferentes; não trocar o Gate atual em um teste de interface.

## Revisão de hardware real (15/10/2026)

O all-in-one adquirido possui um **Intel i3-2100 Sandy Bridge, 2 núcleos / 4 threads**, memória DDR3 de **8 GB** e SSD de **120 GB**. É adequado para desenvolvimento funcional, PostgreSQL de laboratório e pequenas aplicações de baixa demanda; **não** deve ser tratado como máquina de 32 GiB com quatro núcleos físicos.

A implementação do produto será independente dessa limitação: priorizar arquiteturas de `limits/quotas` configuráveis, fila de builds e instâncias de armazenamento isoladas. O perfil deste equipamento deve limitar a execução, não retirar funcionalidades do código.

**Orçamento inicial indicativo, não configuração já aplicada:**

| Área | Meta de memória na fase de laboratório |
| --- | --- |
| SO + cache de disco + margem | reservar aproximadamente 2–3 GiB |
| MackNa Agent + Gate + Proxy | até ~512 MiB somados, a conferir após medição |
| PostgreSQL de laboratório | teto inicial de 768 MiB–1 GiB |
| Redis opcional | teto inicial de 128–256 MiB |
| 1 app experimental Node/Go | teto inicial 256–512 MiB |
| Builder isolado | teto temporário ~1 GiB, **somente 1 job por vez** |
| Proteção contra picos | manter memória disponível e espaço livre em disco; parar testes ao degradar |

Esses números **não** são uma garantia de capacidade. Estabelecer limites Docker/systemd após inspecionar consumo real e escolha do sistema operacional. Em armazenamento, **manter 15–20 GiB livres**; builds, cache, volumes PostgreSQL, WAL, imagens e arquivos temporários precisam de limpeza controlada. **Nunca guardar a única cópia do backup no SSD de 120 GB**.

O i3-2100 não tem AES-NI nem AVX2, de acordo com a Intel. Compilar `GOAMD64=v1` para assegurar compatibilidade genérica x86-64; criptografia AES funcionará por software, com menos desempenho. Verificar imagens Docker de terceiros que dependam de extensões modernas.

Antes da implantação, executar `bash scripts/node02-capacity-report.sh --report` e `bash scripts/v2-node02-admission.sh --check`. Ambos são **somente leitura**.

## Requisitos sugeridos para expansão posterior
- Processador x86-64 com virtualização e AES-NI, preferencialmente uma geração moderna; conferir modelo e desempenho sustentado.
- O SSD atual de 120 GB permite laboratório leve. Para múltiplos projetos e bancos, **500 GB ou 1 TB de SSD** é uma expansão importante, condicionada à compatibilidade do all-in-one. Planejar backups externos.
- RAM atual 8 GiB; consultar placa-mãe e slots para avaliar eventual upgrade para 16 GiB ou 32 GiB. O máximo teórico de 32 GiB do processador não garante suporte da placa. Preferir Ethernet cabeada, BIOS com power-on after AC loss, refrigeração adequada para 24/7.
- Debian 13 amd64 instalado nativamente ou Ubuntu Server LTS suportado, sem host Docker dentro de uma VM para produção, salvo necessidade deliberada de virtualização.
- Nobreak/UPS apropriado, alerta de falhas, sincronismo NTP e DHCP reservado.
- Nunca abrir o Docker Socket, `127.0.0.1:8090`, PostgreSQL de laboratório ou APIs administrativas publicamente.

## Etapa 1: homologar ao máximo na BTV11, com guarda-corpos
1. Preservar `mackna-btv-app-1` e `mackna-btv-db-1` sem `exec`, restart, alterações ou teste de carga direto no PostgreSQL da IMORTAIS.
2. Executar fluxo funcional: login e revogação de token; inventário de projetos; CPU/memória; logs com token apropriado; PostgreSQL Lab somente leitura; 7 relatórios; auditar ações como admin.
3. Usar teste leve e repetível **apenas no health do MackNa**:
   ```bash
   DURATION=60 CONCURRENCY=2 bash /root/mackna-pr4-node01-arm64/scripts/probe-node01.sh
   ```
   Limites do script: 10–300 s e 1–4 workers. Não aumentá-los enquanto app/DB da IMORTAIS coexistirem; interromper em caso de temperatura alta, RAM baixa, I/O travando ou container unhealthy.
4. Antes/depois, apenas inspecionar `docker ps`, `free -m`, `df -h`, `systemctl is-active mackna-agent`, `journalctl -u mackna-agent`. Não usar carga máxima sintética nesta TV box.
5. Exercitar rollback do **MackNa Agent exclusivamente** com snapshot/backup do próprio executável, se autorizado; não alterar Gate, Docker ou volumes.
6. Documentar latência p50/p95 e comportamento do painel em cenários de erro, mas não confundir health endpoint com benchmark de banco ou capacidade máxima de hospedagem.

## Etapa 2: qualificar fisicamente o Node 02
- Confirmar `uname -m` (`x86_64` = AMD64), `lscpu`, `free -h`, `lsblk -o NAME,SIZE,TYPE,FSTYPE,MOUNTPOINTS`, `df -hT`, `cat /etc/os-release`.
- Somente após receber o hardware, executar `bash scripts/preflight-new-node.sh` **como diagnóstico somente leitura**.
- Verificar espaço, SMART/temperatura, rede, UPS e versão Docker.
- Separar discos/partições e limites por serviço. Reservar espaço para sistema, WAL do PostgreSQL, cache/volumes e atualizações, sem usar todo SSD como storage de contêineres.
- Não copiar `/var/run/docker.sock` ou o Docker Gate da BTV11 diretamente; instalar e validar novo Gate específico para AMD64 após revisão de segurança.
- Não utilizar `scripts/install-node.sh` ou `install-verified-bundle.sh` da BTV11 no Node 02: atualmente exigem ARM64 e upgrade de host existente.

## Etapa 3: implantação limpa e paralela
1. Instalar Linux nativo, Docker e o MackNa AMD64 com usuários e systemd mínimos **mediante instalador de primeira instalação próprio**, ainda pendente.
2. Habilitar Gate apenas com a allowlist read-only existente e autenticação do controller via loopback + túnel SSH.
3. Manter nomes de máquina, IP e credenciais do novo nó distintos; criar tokens novos em vez de copiar tokens antigos para outro servidor.
4. Transferir o esquema/fixture do laboratório PostgreSQL por **dump lógico validado (pg_dump/pg_restore)** para um volume isolado novo, não copiar arquivos físicos do volume Docker ARM64. Testar contagem e consultas após restauração.
5. Validar `/api/v1/health`, `/projects`, `/postgres/lab`, `/audit`, limites de memória, timestamps e recuperação após reboot.
6. Executar teste de capacidade progressivo **somente com serviços descartáveis** e limiares de parada. Registrar consumo real e estabelecer quotas. Não migrar produção sem critérios de aceite.

## Etapa 4: decidir o que realmente será migrado
**Distinção obrigatória:**
- **MackNa Cloud e laboratório:** podem migrar primeiro. Melhor reconstruir configuração idempotente e migrar apenas dados experimentais necessários.
- **Containers de homologação da IMORTAIS na BTV11:** só migrar se o usuário autorizar explicitamente, após inventário de volumes, configurações e rollback.
- **IMORTAIS real hospedado no Railway:** não muda automaticamente. Exige projeto separado, plano de janelas de manutenção, backup íntegro, smoke tests, DNS/certificados, monitoramento, rollback e aprovação.
- NUNCA remover os dados da BTV11 antes de ao menos uma restauração comprovada e um período de observação no Node 02.

## Critérios para desativar a BTV11
- Serviços selecionados no novo nó reproduzem o estado esperado em testes funcionais e de segurança.
- Os dados exigidos foram restaurados com hash e contagens verificados; backup externo recuperável.
- Monitoramento ativo, UPS e espaço em disco validados, operação após reboot testada.
- O rollback para BTV11 está definido e documentado, se o nó antigo ainda for mantido como contingência.
- **Só desligar a BTV11 após confirmar que ela não serve tráfego real.**

## Escopo técnico imediato
A branch `feat/node02-amd64-migration-20261015` acrescenta:
- CI **AMD64 de candidato** ao lado do existente ARM64;
- verificador cross-platform `scripts/verify-cross-arch-bundle.sh`;
- este plano de migração.

**Nenhum comando de alteração no Node 01/02 é executado pelo GitHub Actions.**
