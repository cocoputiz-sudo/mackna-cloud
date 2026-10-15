#!/usr/bin/env bash
# Opt-in MackNa Agent updater. No Docker, Gate, PostgreSQL or IMORTAIS commands.
# Only installs a published GitHub Release from the fixed owner/repository.
set -Eeuo pipefail
readonly REPO="cocoputiz-sudo/mackna-cloud"
readonly AGENT="/usr/local/bin/mackna-agent"
readonly SERVICE="mackna-agent.service"
readonly HEALTH="http://127.0.0.1:8090/api/v1/health"
readonly API="https://api.github.com/repos/${REPO}/releases/latest"
mode="${1:---check}"
[[ "$mode" == "--check" || "$mode" == "--apply" || "$mode" == "--enable-auto" || "$mode" == "--disable-auto" ]] || {
 echo "Uso: $0 --check | --apply | --enable-auto | --disable-auto" >&2; exit 2;
}
[[ $(id -u) == 0 && $(uname -m) == aarch64 ]] || { echo "Exige root em Linux ARM64" >&2; exit 2; }
if [[ "$mode" == "--enable-auto" ]]; then
 install -o root -g root -m 0755 "$0" /usr/local/sbin/mackna-agent-updater
 cat >/etc/systemd/system/mackna-agent-autoupdate.service <<'UNIT'
[Unit]
Description=MackNa Agent - atualização de release verificada
Wants=network-online.target
After=network-online.target
[Service]
Type=oneshot
ExecStart=/usr/local/sbin/mackna-agent-updater --apply
TimeoutStartSec=180
UNIT
 cat >/etc/systemd/system/mackna-agent-autoupdate.timer <<'TIMER'
[Unit]
Description=Verificar release estável do MackNa Agent a cada 12 horas
[Timer]
OnCalendar=*-*-* 04,16:00:00
RandomizedDelaySec=30m
Persistent=true
Unit=mackna-agent-autoupdate.service
[Install]
WantedBy=timers.target
TIMER
 systemctl daemon-reload
 systemctl enable --now mackna-agent-autoupdate.timer
 echo "Timer instalado. Publicação de releases é manual no GitHub; apenas agentes com checksum válido podem ser aplicados."
 exit 0
fi
if [[ "$mode" == "--disable-auto" ]];then
 systemctl disable --now mackna-agent-autoupdate.timer
 echo "Atualizações automáticas desativadas."
 exit 0
fi
for bin in curl python3 sha256sum install mktemp cp mv rm systemctl flock;do
 command -v "$bin" >/dev/null || { echo "Dependência ausente: $bin" >&2; exit 2; }
done
# Prevent concurrent manual and timer attempts.
exec 9>/run/mackna-agent-updater.lock
flock -n 9 || { echo "Atualização já em execução" >&2; exit 1; }
work=$(mktemp -d /tmp/mackna-release.XXXXXXXX)
staged=
backup=
swapped=0
done_ok=0
cleanup(){
 local rc=$?
 trap - ERR EXIT
 if (( swapped == 1 && done_ok == 0 ));then
  echo "Falha na validação: restaurando somente o MackNa Agent anterior." >&2
  if [[ -n "$backup" && -s "$backup" ]];then
   install -o root -g root -m 0755 "$backup" "$AGENT" || true
   systemctl restart "$SERVICE" || true
  fi
 fi
 [[ -z "$staged" ]] || rm -f "$staged"
 rm -rf "$work"
 exit "$rc"
}
trap cleanup EXIT
curl --proto '=https' --tlsv1.2 -fsSL --retry 2 --max-time 30 \
 -H "Accept: application/vnd.github+json" "$API" -o "$work/release.json"
# Strongly restrict release tag to a stable version; no arbitrary URL input.
tag=$(python3 - "$work/release.json" <<'PY'
import json,re,sys
with open(sys.argv[1],encoding="utf-8") as f: data=json.load(f)
tag=data.get("tag_name","")
if data.get("draft") or data.get("prerelease") or not re.fullmatch(r"agent-v[0-9]+\.[0-9]+\.[0-9]+",tag):
 raise SystemExit("No valid stable MackNa Agent release available")
required={"mackna-agent-linux-arm64","SHA256SUMS","BUILD_INFO.txt"}
present={a.get("name") for a in data.get("assets",[])}
if not required.issubset(present):raise SystemExit("Release missing mandatory assets")
print(tag)
PY
)
readonly base="https://github.com/${REPO}/releases/download/${tag}"
for asset in mackna-agent-linux-arm64 SHA256SUMS BUILD_INFO.txt;do
 curl --proto '=https' --tlsv1.2 -fsSL --retry 2 --max-time 120 \
  "$base/$asset" -o "$work/$asset"
done
release_sha=$(python3 - "$work/BUILD_INFO.txt" "$work/SHA256SUMS" <<'PY'
import hashlib,pathlib,re,sys
values=dict(line.strip().split("=",1) for line in pathlib.Path(sys.argv[1]).read_text().splitlines() if "=" in line)
sha=values.get("source_sha","")
if not re.fullmatch("[0-9a-f]{40}",sha):raise SystemExit("Invalid source SHA in manifest")
checks=pathlib.Path(sys.argv[2]).read_text().splitlines()
expected=[line.split("  ")[0] for line in checks if line.endswith("  mackna-agent-linux-arm64")]
if len(expected)!=1 or not re.fullmatch("[0-9a-f]{64}",expected[0]):raise SystemExit("Invalid SHA256SUMS")
binary=pathlib.Path(sys.argv[2]).parent/"mackna-agent-linux-arm64"
if hashlib.sha256(binary.read_bytes()).hexdigest()!=expected[0]:raise SystemExit("Binary SHA256 mismatch")
data=binary.read_bytes()
if b"postgres_lab.js" not in data or b"script-src 'self'; connect-src 'self'" not in data or sha.encode() not in data:
 raise SystemExit("Binary lacks required build fingerprint")
# ELF e_machine=183 is AArch64 (little endian).
if data[:4]!=b"\x7fELF" or int.from_bytes(data[18:20],"little")!=183:
 raise SystemExit("Release binary is not ARM64")
print(sha)
PY
)
echo "Release validada: $tag · source=$release_sha"
if [[ -x "$AGENT" ]] && cmp -s "$AGENT" "$work/mackna-agent-linux-arm64";then
 echo "MackNa Agent já corresponde ao release. Nenhuma alteração.";done_ok=1; exit 0
fi
if [[ "$mode" == "--check" ]];then
 echo "Nova atualização disponível. Nenhum serviço foi reiniciado."
 echo "Para atualizar manualmente: sudo /usr/local/sbin/mackna-agent-updater --apply"
 done_ok=1; exit 0
fi
systemctl is-active --quiet "$SERVICE" || { echo "Agent inativo: atualização recusada" >&2; exit 1; }
curl -fsS --max-time 3 "$HEALTH" >/dev/null || { echo "Health pré-update falhou" >&2; exit 1; }
backup="$work/previous-agent"
cp -p "$AGENT" "$backup"
staged=$(mktemp /usr/local/bin/.mackna-agent.next.XXXXXXXX)
install -o root -g root -m 0755 "$work/mackna-agent-linux-arm64" "$staged"
mv -f "$staged" "$AGENT";staged=
swapped=1
systemctl restart "$SERVICE"
health_ok=0
for i in $(seq 1 15);do
 if curl -fsS --max-time 3 "$HEALTH" -o "$work/health.json" 2>/dev/null;then
  if python3 - "$work/health.json" "$release_sha" <<'PY'
import json,sys
data=json.load(open(sys.argv[1]))
assert data.get("ok") is True and data.get("build")==sys.argv[2]
PY
  then health_ok=1;break;fi
 fi
 sleep 1
done
[[ "$health_ok" == 1 ]] || { echo "Agent atualizado não confirmou commit/health" >&2; exit 1; }
install -o root -g root -m 0755 "$backup" "$AGENT.previous"
done_ok=1
echo "MackNa Agent atualizado para $tag ($release_sha). IMORTAIS, Gate e PostgreSQL não foram reiniciados."
