# MackNa Cloud · Entrega unificada ao Node 01 (BTV11)

> Estado: candidato à instalação, requer CI verde e autorização operacional. Não executar esta documentação em produção sem conferir o SHA-256.

## O que será instalado
- **Somente o MackNa Agent** (binário Go ARM64, painel Node 01 e projetos, PostgreSQL Manager laboratório).
- O Gate Docker em execução, Docker Engine e os contêineres `mackna-btv-*` **não serão reiniciados ou alterados**.
- A configuração PostgreSQL Lab existente em `/etc/mackna/pg-lab.env` é preservada; não executar de novo `activate-pg-lab.sh`.
- O atualizador de releases é incluído como arquivo para posterior revisão. **Timer de atualização automática não é ativado**.

## Antes de começar
1. BTV11 acessível por SSH em `root@192.168.15.11`.
2. Sistema operacional ARM64 e `mackna-agent.service` ativo.
3. Laboratório `mackna-pg-lab` healthy, volume `mackna_pg_lab_data` preservado.
4. Arquivo ZIP publicado por GitHub Actions e verificado: contém `dist/mackna-agent-linux-arm64`, `dist/mackna-gate-linux-arm64` (não instalar Gate), `dist/SHA256SUMS`, `dist/BUILD_INFO.txt`, `scripts/install-verified-bundle.sh`, `scripts/upgrade-agent.sh`.

## Transferir um único ZIP do Windows

No PowerShell do Windows, usando o **nome exato** do ZIP baixado:
```powershell
$zip = "$env:USERPROFILE\Downloads\NOME_EXATO_DO_ZIP.zip"
Test-Path $zip
scp $zip root@192.168.15.11:/root/mackna-bundle.zip
```
Na BTV11:
```bash
mkdir -p /root/mackna-candidate
unzip -q -o /root/mackna-bundle.zip -d /root/mackna-candidate
bash /root/mackna-candidate/scripts/install-verified-bundle.sh /root/mackna-candidate --verify-only
```
**Se falhar a verificação, parar e não instalar.** O verificador não instala nem reinicia nada.

## Instalar somente o agente
Após conferir o nome do build/commit e autorizar a instalação:
```bash
bash /root/mackna-candidate/scripts/install-verified-bundle.sh /root/mackna-candidate --install
```

A instalação guarda o binário anterior, reinicia **apenas `mackna-agent.service`**, exige health HTTP 200 e commit esperado. Em falha faz rollback somente do agente.

## Homologação pós-instalação
```bash
systemctl is-active mackna-agent
curl --fail --silent --show-error http://127.0.0.1:8090/api/v1/health
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
```
O JSON de health deve conter `"ok":true` e `"build":"<SHA do pacote>"`. Conferir o Docker somente por inspeção. **Não enviar tokens ou senhas em capturas.**

Do Windows, em PowerShell separado:
```powershell
ssh -N -L 8090:127.0.0.1:8090 root@192.168.15.11
```
Acessar:
- `http://127.0.0.1:8090/`: Node 01, Docker e serviços.
- `http://127.0.0.1:8090/projects`: inventário de projetos.
- `http://127.0.0.1:8090/postgres/lab`: PostgreSQL de laboratório. Usuário Basic: `admin`, senha: token MackNa válido.
- Testar métricas, `public.mackna_fixture`, colunas, índices e relacionamentos.
- Verificar explicitamente que `mackna-btv-app-1` e `mackna-btv-db-1` continuam saudáveis. **Nunca conectar o manager ao banco IMORTAIS nesta fase.**

## Emergência e rollback manual do Agent
Se o verificador de saúde não conseguir fazer rollback:
```bash
install -o root -g root -m 0755 /usr/local/bin/mackna-agent.previous /usr/local/bin/mackna-agent
systemctl restart mackna-agent
systemctl is-active mackna-agent
```
Não executar remoções de volumes, `docker compose down` nem alterações na IMORTAIS.

## Ainda não será disponibilizado amanhã
| Funcionalidade | Estado |
| --- | --- |
| Métricas, contêineres e projetos Docker | Funcional, consulta |
| Logs administrativos sob demanda | Funcional, permissão logs/admin |
| PostgreSQL Lab Manager com tabelas e métricas | Funcional, leitura |
| Pacote e rollback do Agent | Implementado, teste no host amanhã |
| Release updater do GitHub | Código preparado, requer release oficial e homologação |
| Deploy de projetos, criação de bancos reais, CRUD, backups/restore | Pendente de implementação e isolamento |
| Domínios/HTTPS, gestão multi-nó, agentes IA com autorização | Pendente; não expor à internet |

**O MackNa ainda não substitui Railway nem constitui serviço de produção.**
