# Node 01: implantação segura do primeiro agente

> **Ainda não executar na BTV11 sem revisar o PR e baixar o artefato após CI aprovado.**
> Esta etapa **não altera** os contêineres da IMORTAIS nem o Railway de produção.

## 1. Pré-requisitos

- Debian 13 ARM64 inicializando pela eMMC, Docker Engine operacional.
- Imagens do PostgreSQL e IMORTAIS já homologadas de forma independente na BTV11.
- Um artefato `mackna-agent-linux-arm64` produzido pelo GitHub Actions (não compilar na BTV11).
- Acesso SSH como usuário com acesso root via `su -`.

## 2. Baixar o binário produzido pelo Actions

No GitHub, abra **Actions > MackNa Cloud MVP > execução aprovada > Artifacts > mackna-agent-linux-arm64**.

Extraia o ZIP no PC; verifique se contém o executável `mackna-agent-linux-arm64`.

A partir do PowerShell no Windows, ajuste o caminho local e envie **apenas o binário**:

```powershell
scp .\mackna-agent-linux-arm64 imortais@192.168.15.11:/tmp/mackna-agent
```

## 3. Instalar o serviço (na BTV11, como root)

Transfira também o arquivo `deploy/systemd/mackna-agent.service` para `/tmp/mackna-agent.service` via SCP, ou copie-o após clonar o repositório em um diretório independente.

```bash
su -
groupadd --system mackna 2>/dev/null || true
id mackna >/dev/null 2>&1 || useradd --system --no-create-home --shell /usr/sbin/nologin -g mackna -G docker mackna
install -o root -g root -m 0755 /tmp/mackna-agent /usr/local/bin/mackna-agent
install -d -o root -g mackna -m 0750 /etc/mackna
umask 077
printf 'MACKNA_ADMIN_TOKEN=%s\n' "$(openssl rand -hex 32)" > /etc/mackna/mackna.env
chown root:mackna /etc/mackna/mackna.env
chmod 0640 /etc/mackna/mackna.env
install -o root -g root -m 0644 /tmp/mackna-agent.service /etc/systemd/system/mackna-agent.service
systemctl daemon-reload
systemctl enable --now mackna-agent
systemctl --no-pager status mackna-agent
curl -i http://127.0.0.1:8090/api/v1/health
```

**Atenção:** o grupo `docker` concede acesso efetivamente privilegiado ao daemon. Este agente executa só métodos GET, sem shell, deploy ou edição. Ainda assim, o endpoint precisa ficar estritamente em loopback, e a conta não deve ser utilizada para outras finalidades. O endurecimento futuro deverá reduzir essa dependência.

O arquivo `/etc/mackna/mackna.env` contém um segredo: **não publique seu conteúdo, não cole no chat e não faça commit**.

## 4. Acessar o painel pelo Windows

No PowerShell de seu PC, deixe este processo aberto:

```powershell
ssh -N -L 8090:127.0.0.1:8090 imortais@192.168.15.11
```

Abra `http://127.0.0.1:8090/` no navegador. Ele solicitará login HTTP Basic:

- Usuário: `admin`
- Senha: o valor de `MACKNA_ADMIN_TOKEN` lido **localmente** pelo administrador no servidor.

Não utilize HTTP Basic em uma interface exposta diretamente na LAN ou na internet; o SSH fornece a proteção de transporte no MVP.

## 5. Critérios de aceite

- O painel informa hostname, arquitetura, RAM, disco, uptime e load average.
- São listados os contêineres existentes, inclusive os do projeto `mackna-btv`.
- Sem login, `/api/v1/system` e `/api/v1/containers` respondem 401.
- `POST` ou outros métodos de escrita respondem 405.
- O serviço não reinicia, altera, recria ou remove nenhum contêiner.
- O healthcheck `/api/v1/health` responde 200.
- Nenhum serviço da IMORTAIS no Railway foi modificado.
- Monitorar memória, disco e logs por 30 minutos antes de considerar homologado.

## 6. Rollback

```bash
systemctl disable --now mackna-agent
```

Isso apenas desativa o novo agente, sem alterar os serviços Docker existentes.
