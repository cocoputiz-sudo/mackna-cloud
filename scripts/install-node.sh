#!/usr/bin/env bash
# MackNa Cloud PR2 read-only install on Debian 13 ARM64, run as root.
set -euo pipefail
[ "$(id -u)" = 0 ] || { echo "Run as root (su -)" >&2; exit 1; }
AGENT="${1:-./mackna-agent-linux-arm64}"
GATE="${2:-./mackna-gate-linux-arm64}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE="$(cd "$HERE/.." && pwd)"
[ -x "$AGENT" ] || { echo "Agent binary missing: $AGENT" >&2; exit 1; }
[ -x "$GATE" ] || { echo "Gate binary missing: $GATE" >&2; exit 1; }
[ "$(uname -m)" = "aarch64" ] || { echo "This installer is for ARM64" >&2; exit 1; }
getent group docker >/dev/null || { echo "Docker group missing" >&2; exit 1; }
getent group mackna >/dev/null || groupadd --system mackna
id mackna >/dev/null 2>&1 || useradd --system --gid mackna --no-create-home --home-dir /nonexistent --shell /usr/sbin/nologin mackna
getent group mackna-gate >/dev/null || groupadd --system mackna-gate
id mackna-gate >/dev/null 2>&1 || useradd --system --gid mackna-gate --no-create-home --home-dir /nonexistent --shell /usr/sbin/nologin mackna-gate
usermod -aG docker,mackna mackna-gate
install -o root -g root -m 0755 "$AGENT" /usr/local/bin/.mackna-agent.next
install -o root -g root -m 0755 "$GATE" /usr/local/bin/.mackna-gate.next
mv -f /usr/local/bin/.mackna-agent.next /usr/local/bin/mackna-agent
mv -f /usr/local/bin/.mackna-gate.next /usr/local/bin/mackna-gate
install -d -o mackna -g mackna -m 0700 /var/lib/mackna
install -o root -g root -m 0644 "$BASE/deploy/systemd/mackna-gate.service" /etc/systemd/system/mackna-gate.service
install -o root -g root -m 0644 "$BASE/deploy/systemd/mackna-agent.service" /etc/systemd/system/mackna-agent.service
systemctl daemon-reload
systemctl enable --now mackna-gate.service
systemctl enable --now mackna-agent.service
systemctl restart mackna-gate.service mackna-agent.service
systemctl --no-pager --full status mackna-gate.service mackna-agent.service || true
echo "Node health: curl -i http://127.0.0.1:8090/api/v1/health"
echo "To create a token as root: su -s /bin/sh mackna -c '/usr/local/bin/mackna-agent token create admin admin'"
echo "Never publish the token. Dashboard access requires SSH forwarding."
