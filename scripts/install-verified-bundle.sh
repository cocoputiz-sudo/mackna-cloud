#!/usr/bin/env bash
# MackNa Cloud: verify an ARM64 GitHub Actions artifact and optionally install
# ONLY the controller. Never changes Docker, Gate, PostgreSQL or IMORTAIS.
set -Eeuo pipefail

usage() {
  echo "Uso: bash scripts/install-verified-bundle.sh PASTA_EXTRAIDA --verify-only|--install" >&2
}
if [[ $# -ne 2 ]]; then usage; exit 2; fi
BUNDLE="${1%/}"
ACTION="$2"
if [[ "$ACTION" != "--verify-only" && "$ACTION" != "--install" ]]; then usage; exit 2; fi

DIST="$BUNDLE/dist"
AGENT="$DIST/mackna-agent-linux-arm64"
MANIFEST="$DIST/SHA256SUMS"
INFO="$DIST/BUILD_INFO.txt"
[[ -f "$AGENT" && -f "$MANIFEST" && -f "$INFO" ]] || {
  echo "Pacote incompleto: esperado dist/{mackna-agent-linux-arm64,SHA256SUMS,BUILD_INFO.txt}" >&2
  exit 1
}
for cmd in sha256sum grep sed od; do command -v "$cmd" >/dev/null || {
  echo "Dependência ausente: $cmd" >&2; exit 1;
}; done

echo "=== VALIDANDO O PACOTE ==="
(cd "$DIST" && sha256sum -c SHA256SUMS)
SOURCE_SHA=$(sed -n 's/^source_sha=//p' "$INFO")
[[ "$SOURCE_SHA" =~ ^[a-f0-9]{40}$ ]] || {
  echo "Manifesto sem commit Git válido" >&2; exit 1;
}
MAGIC=$(od -An -tx1 -N4 "$AGENT" | tr -d ' \n')
MACHINE=$(od -An -tu2 -j18 -N2 "$AGENT" | tr -d ' \n')
[[ "$MAGIC" == 7f454c46 && "$MACHINE" == 183 ]] || {
  echo "Binário não é Linux ELF ARM64" >&2; exit 1;
}
for marker in "$SOURCE_SHA" "postgres_lab.js" "script-src 'self'; connect-src 'self'"; do
  if ! grep -aqF "$marker" "$AGENT"; then
    echo "O executável não contém a assinatura esperada: $marker" >&2
    exit 1
  fi
done
printf 'Commit autenticado pelo manifesto: %s\n' "$SOURCE_SHA"
sha256sum "$AGENT"
echo "Binário e manifesto conferidos. Nenhuma operação de banco executada."
if [[ "$ACTION" == "--verify-only" ]]; then exit 0; fi

[[ $(id -u) == 0 ]] || { echo "Instalação requer root" >&2; exit 1; }
[[ $(uname -m) == aarch64 ]] || { echo "Instalação permitida somente na BTV11 ARM64" >&2; exit 1; }
for cmd in systemctl curl cp install mktemp rm sleep; do command -v "$cmd" >/dev/null || {
  echo "Dependência ausente: $cmd" >&2; exit 1;
}; done
[[ -f "$BUNDLE/scripts/upgrade-agent.sh" ]] || {
  echo "Script de upgrade ausente no pacote" >&2; exit 1;
}
bash -n "$BUNDLE/scripts/upgrade-agent.sh"
systemctl is-active --quiet mackna-agent || {
  echo "Agente inativo. Abortado antes de alterar arquivos." >&2; exit 1;
}
curl -fsS --noproxy '*' --max-time 3 http://127.0.0.1:8090/api/v1/health >/dev/null || {
  echo "Agente sem saúde antes da atualização. Abortado." >&2; exit 1;
}

# Preserve the original binary independently of the inner updater's backup.
BACKUP=$(mktemp /usr/local/bin/.mackna-agent.before-bundle.XXXXXXXX)
cp -p /usr/local/bin/mackna-agent "$BACKUP"
NEEDS_ROLLBACK=1
finish() {
  local rc=$?
  trap - EXIT
  if [[ "$NEEDS_ROLLBACK" == 1 && $rc -ne 0 ]]; then
    echo "ERRO na homologação. Restaurando apenas o MackNa Agent anterior..." >&2
    install -o root -g root -m 0755 "$BACKUP" /usr/local/bin/mackna-agent || true
    systemctl restart mackna-agent || true
  fi
  rm -f "$BACKUP"
  exit "$rc"
}
trap finish EXIT

bash "$BUNDLE/scripts/upgrade-agent.sh" "$AGENT"
CHECK_OK=0
for attempt in 1 2 3 4 5 6 7 8 9 10; do
  result=$(curl -fsS --noproxy '*' --max-time 3 http://127.0.0.1:8090/api/v1/health 2>/dev/null || true)
  if [[ "$result" == *'"ok":true'* && "$result" == *"\"build\":\"$SOURCE_SHA\""* ]]; then
    CHECK_OK=1
    echo "Agente identificado: $result"
    break
  fi
  sleep 1
done
[[ "$CHECK_OK" == 1 ]] || {
  echo "Agente não confirmou o commit esperado após instalação. Reversão acionada." >&2
  exit 1
}
NEEDS_ROLLBACK=0
echo "Atualização homologada. Docker, Gate e serviços da IMORTAIS não foram reiniciados."
