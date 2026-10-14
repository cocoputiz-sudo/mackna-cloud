#!/usr/bin/env bash
# PR4 opt-in lab activation. Never touches IMORTAIS containers, networks or volumes.
set -Eeuo pipefail
[[ $(id -u) == 0 ]] || { echo 'Execute como root' >&2; exit 1; }
[[ $(uname -m) == aarch64 ]] || { echo 'Somente ARM64' >&2; exit 1; }
for cmd in docker openssl systemctl curl install mktemp; do command -v "$cmd" >/dev/null || { echo "Falta: $cmd" >&2; exit 1; }; done
[[ $(docker inspect -f '{{.State.Health.Status}}' mackna-pg-lab 2>/dev/null) == healthy ]] || { echo 'Lab não está healthy' >&2; exit 1; }
[[ $(docker inspect -f '{{.HostConfig.NetworkMode}}' mackna-pg-lab) == none ]] || { echo 'Lab não está no isolamento esperado. Interrompido.' >&2; exit 1; }
[[ ! -e /etc/systemd/system/mackna-agent.service.d/40-pg-lab.conf ]] || { echo 'Lab já configurado; interrompido para evitar sobrescrita.' >&2; exit 1; }
docker network inspect mackna-pg-lab-net >/dev/null 2>&1 || docker network create --driver bridge --internal mackna-pg-lab-net >/dev/null
docker network connect mackna-pg-lab-net mackna-pg-lab
ip=$(docker inspect -f '{{(index .NetworkSettings.Networks "mackna-pg-lab-net").IPAddress}}' mackna-pg-lab)
[[ "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'IP inválido' >&2; exit 1; }
password=$(openssl rand -hex 24)
# SQL receives password as a psql variable, not interpolated SQL.
docker exec -i mackna-pg-lab psql -v ON_ERROR_STOP=1 -U postgres -d mackna_lab -v "newpass=$password" <<'SQL'
ALTER ROLE mackna_reader WITH PASSWORD :'newpass';
ALTER ROLE mackna_reader SET default_transaction_read_only = on;
ALTER ROLE mackna_reader SET statement_timeout = '15s';
SQL
mkdir -p /etc/mackna /etc/systemd/system/mackna-agent.service.d
chmod 700 /etc/mackna
tmp=$(mktemp /etc/mackna/.pg-lab.XXXXXX)
chmod 600 "$tmp"
printf 'MACKNA_PG_LAB_ENABLED=1\nMACKNA_PG_LAB_DSN=postgres://mackna_reader:%s@%s:5432/mackna_lab?sslmode=disable\n' "$password" "$ip" >"$tmp"
mv "$tmp" /etc/mackna/pg-lab.env
unset password
cat >/etc/systemd/system/mackna-agent.service.d/40-pg-lab.conf <<'UNIT'
[Service]
EnvironmentFile=/etc/mackna/pg-lab.env
UNIT
systemctl daemon-reload
systemctl restart mackna-agent
for i in 1 2 3 4 5 6 7 8 9 10; do
 if curl --fail --silent --output /dev/null --max-time 2 http://127.0.0.1:8090/api/v1/health; then break; fi
 sleep 1
done
if ! curl --fail --silent --output /dev/null --max-time 3 http://127.0.0.1:8090/api/v1/health; then
 echo 'Agent falhou: desabilitando Lab e reiniciando agent' >&2
 rm -f /etc/systemd/system/mackna-agent.service.d/40-pg-lab.conf
 systemctl daemon-reload
 systemctl restart mackna-agent
 exit 1
fi
echo 'Lab configurado; acesso exige token MackNa. Nunca compartilhe /etc/mackna/pg-lab.env.'
docker ps --format 'table {{.Names}}\t{{.Status}}'
