#!/usr/bin/env bash
# PR4 lab-only activation; never modifies mackna-btv-* or its volumes.
set -Eeuo pipefail
LAB=mackna-pg-lab
VOL=mackna_pg_lab_data
DROPIN=/etc/systemd/system/mackna-agent.service.d/40-pg-lab.conf
ENVFILE=/etc/mackna/pg-lab.env
URL=http://127.0.0.1:8090/api/v1/postgres/lab/overview
changed=0
complete=0
password=
response=
cleanup() {
  local rc=$?
  trap - ERR EXIT
  if (( complete == 0 && changed == 1 )); then
    echo "Falha (exit $rc): revertendo exclusivamente o laboratório e o MackNa Agent" >&2
    rm -f "$DROPIN" "$ENVFILE"
    systemctl daemon-reload || true
    systemctl restart mackna-agent || true
    if docker inspect "$LAB" >/dev/null 2>&1; then
      docker rm -f "$LAB" >/dev/null || true
    fi
    docker run -d --name "$LAB" --restart no --network none \
      --memory 192m --cpus 0.50 --pids-limit 64 \
      -e POSTGRES_DB=mackna_lab -e POSTGRES_USER=postgres \
      -e POSTGRES_PASSWORD=lab-rollback-unused \
      -v "$VOL":/var/lib/postgresql/data \
      --health-cmd='pg_isready -U postgres -d mackna_lab' \
      --health-interval=10s --health-timeout=3s --health-retries=5 \
      postgres:16-alpine >/dev/null || echo "ATENÇÃO: restauração do lab falhou" >&2
  fi
  unset password MACKNA_TOKEN
  exit "$rc"
}
trap cleanup ERR EXIT
[[ $(id -u) == 0 && $(uname -m) == aarch64 ]] || { echo "Exige root em ARM64" >&2; exit 1; }
for bin in docker openssl systemctl curl mktemp grep sed seq sleep chmod chown mv rm mkdir; do command -v "$bin" >/dev/null || { echo "Dependência ausente: $bin" >&2; exit 1; }; done
[[ -n "${MACKNA_TOKEN:-}" ]] || { echo "Informe MACKNA_TOKEN no ambiente; nunca no argumento do script" >&2; exit 1; }
[[ ! -e "$DROPIN" && ! -e "$ENVFILE" ]] || { echo "Configuração já existe; não sobrescrevendo" >&2; exit 1; }
[[ $(docker inspect -f '{{.HostConfig.NetworkMode}}' "$LAB") == none ]] || { echo "Lab não está em network none" >&2; exit 1; }
[[ $(docker inspect -f '{{.State.Health.Status}}' "$LAB") == healthy ]] || { echo "Lab não está healthy" >&2; exit 1; }
[[ $(docker inspect -f '{{.Config.Image}}' "$LAB") == postgres:16-alpine ]] || { echo "Imagem inesperada" >&2; exit 1; }
actual_volume=$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/var/lib/postgresql/data"}}{{.Name}}{{end}}{{end}}' "$LAB")
[[ "$actual_volume" == "$VOL" ]] || { echo "Volume divergente: abortado" >&2; exit 1; }
[[ $(docker volume inspect -f '{{.Name}}' "$VOL") == "$VOL" ]] || exit 1
if docker ps --format '{{.Ports}}' | grep -q '127.0.0.1:55432->'; then
 echo "Porta 55432 já está ocupada" >&2; exit 1
fi
docker rm -f "$LAB" >/dev/null
changed=1
docker run -d --name "$LAB" --restart unless-stopped \
 --memory 192m --cpus 0.50 --pids-limit 64 \
 -p 127.0.0.1:55432:5432 \
 -e POSTGRES_DB=mackna_lab -e POSTGRES_USER=postgres \
 -e POSTGRES_PASSWORD=lab-existing-volume-unused \
 -v "$VOL":/var/lib/postgresql/data \
 --health-cmd='pg_isready -U postgres -d mackna_lab' \
 --health-interval=10s --health-timeout=3s --health-retries=5 \
 postgres:16-alpine >/dev/null
healthy=0
for i in $(seq 1 30); do
 if [[ $(docker inspect -f '{{.State.Health.Status}}' "$LAB" 2>/dev/null) == healthy ]]; then healthy=1; break; fi
 sleep 1
done
[[ "$healthy" == 1 ]] || { echo "Lab não ficou healthy" >&2; exit 1; }
password=$(openssl rand -hex 24)
# Password reaches psql only through stdin, not argv or environment.
printf "\\set newpass %s\nALTER ROLE mackna_reader WITH PASSWORD :'newpass';\nALTER ROLE mackna_reader SET default_transaction_read_only = on;\nALTER ROLE mackna_reader SET statement_timeout = '15s';\n" "$password" |
 docker exec -i "$LAB" psql -X -q -v ON_ERROR_STOP=1 -U postgres -d mackna_lab >/dev/null
# Actual host-network TCP authentication, with password supplied via stdin.
probe=$(printf '%s\n' "$password" |
 docker run --rm -i --network host postgres:16-alpine sh -c '
  IFS= read -r PGPASSWORD; export PGPASSWORD
  psql -X -qAt -v ON_ERROR_STOP=1 -h 127.0.0.1 -p 55432 -U mackna_reader -d mackna_lab \
    -c "SELECT 1, current_setting('\''transaction_read_only'\'')"'
 )
[[ "$probe" == '1|on' ]] || { echo "Teste SELECT 1/read-only falhou" >&2; exit 1; }
mkdir -p /etc/mackna /etc/systemd/system/mackna-agent.service.d
tmp=$(mktemp /etc/mackna/.pg-lab.XXXXXX)
chmod 600 "$tmp"
printf 'MACKNA_PG_LAB_ENABLED=1\nMACKNA_PG_LAB_DSN=postgres://mackna_reader:%s@127.0.0.1:55432/mackna_lab?sslmode=disable\n' "$password" >"$tmp"
chown root:root "$tmp"
mv "$tmp" "$ENVFILE"
cat >"$DROPIN" <<'UNIT'
[Service]
EnvironmentFile=/etc/mackna/pg-lab.env
UNIT
systemctl daemon-reload
systemctl restart mackna-agent
for i in $(seq 1 12); do
 if curl -fsS --max-time 3 http://127.0.0.1:8090/api/v1/health >/dev/null; then break; fi
 sleep 1
done
# Avoid exposing bearer token in process argv or shell history: curl config via stdin.
response=$(printf 'header = "Authorization: Bearer %s"\n' "$MACKNA_TOKEN" |
 curl -fsS --max-time 18 -K - "$URL")
# Validate returned JSON includes server version 16, not just HTTP 200.
printf '%s' "$response" | grep -Eq '"version"[[:space:]]*:[[:space:]]*"16([.]|")' || {
 echo "Overview não retornou PostgreSQL 16" >&2; exit 1;
}
complete=1
echo "=== LAB PUBLICADO SOMENTE EM LOOPBACK ==="
docker ps --filter "name=^/${LAB}$" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
echo "=== OVERVIEW AUTENTICADO ==="
printf '%s\n' "$response"
echo "Lab ativado; IMORTAIS não foi alterada."
