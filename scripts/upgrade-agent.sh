#!/usr/bin/env bash
# Agent-only upgrade for PR3. Never restarts Docker, the Gate or IMORTAIS services.
set -Eeuo pipefail

TARGET="/usr/local/bin/mackna-agent"
SERVICE="mackna-agent.service"
HEALTH="http://127.0.0.1:8090/api/v1/health"
if [ "$#" -ne 1 ]; then
 echo "Uso: bash scripts/upgrade-agent.sh dist/mackna-agent-linux-arm64" >&2
 exit 1
fi
SOURCE="$1"

[ "$(id -u)" = "0" ] || { echo "Execute como root (su -)." >&2; exit 1; }
[ "$(uname -m)" = "aarch64" ] || { echo "Este pacote exige ARM64/aarch64." >&2; exit 1; }
[ -s "$SOURCE" ] || { echo "Binário vazio ou ausente: $SOURCE" >&2; exit 1; }
[ -x "$TARGET" ] || { echo "Agent atual ausente; use o instalador inicial." >&2; exit 1; }
for bin in systemctl curl install mktemp cp mv rm sleep; do
 command -v "$bin" >/dev/null || { echo "Dependência ausente: $bin" >&2; exit 1; }
done
systemctl is-active --quiet "$SERVICE" || { echo "Agent não está ativo; upgrade cancelado." >&2; exit 1; }
health_ok() {
 curl --fail --silent --show-error --output /dev/null --max-time 3 "$HEALTH"
}
health_ok || { echo "Saúde do Node indisponível ANTES da atualização; nada foi alterado." >&2; exit 1; }

STAGED="$(mktemp /usr/local/bin/.mackna-agent.next.XXXXXX)"
BACKUP="$(mktemp /usr/local/bin/.mackna-agent.backup.XXXXXX)"
ROLLBACK=0
cleanup() {
 if [ "$ROLLBACK" = 1 ]; then
  echo "Falha no upgrade. Restaurando binário anterior do agent..." >&2
  cp -p "$BACKUP" "$STAGED" && mv -f "$STAGED" "$TARGET" || true
  systemctl restart "$SERVICE" || true
 fi
 rm -f "$STAGED" "$BACKUP"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

install -o root -g root -m 0755 "$SOURCE" "$STAGED"
cp -p "$TARGET" "$BACKUP"
ROLLBACK=1
mv -f "$STAGED" "$TARGET"
systemctl restart "$SERVICE"
HEALTHY=0
for attempt in 1 2 3 4 5 6 7 8 9 10; do
 if health_ok; then HEALTHY=1; break; fi
 sleep 1
done
if [ "$HEALTHY" != 1 ]; then
 echo "Agent não retornou saúde HTTP 200 após o upgrade." >&2
 exit 1
fi
install -o root -g root -m 0755 "$BACKUP" "$TARGET.previous"
ROLLBACK=0
echo "MackNa agent atualizado com sucesso. Backup anterior: $TARGET.previous"
echo "Gate, Docker, PostgreSQL e backend IMORTAIS não foram reiniciados."
