#!/usr/bin/env bash
# Read-only sizing and OS compatibility probe for a larger MackNa Cloud node.
# NEVER starts/stops Docker, containers, PostgreSQL, Gate, or systemd units.
set -Eeuo pipefail
printf '=== MACKNA NODE PREFLIGHT (READ ONLY) ===\n'
printf 'UTC: '; date -u +%FT%TZ
printf 'Arquitetura: '; uname -m
printf 'Kernel: '; uname -r
if [[ -r /etc/os-release ]];then
 . /etc/os-release
 printf 'Sistema: %s\n' "${PRETTY_NAME:-unknown}"
fi
printf '\n=== RECURSOS ===\n'
printf 'CPU lógicas: '; getconf _NPROCESSORS_ONLN
if command -v free >/dev/null;then free -h;fi
df -hT / /var/lib/docker 2>/dev/null || df -hT /
printf '\n=== AMBIENTE ===\n'
for binary in docker curl systemctl python3 openssl sha256sum;do
 if command -v "$binary" >/dev/null;then printf '%s: instalado\n' "$binary";else printf '%s: ausente\n' "$binary";fi
done
if command -v docker >/dev/null;then
 docker version --format 'Docker Server: {{.Server.Version}}' 2>/dev/null || echo 'Docker sem conexão local'
 echo 'Containers (somente nome e estado):'
 docker ps --format 'table {{.Names}}\t{{.Status}}' 2>/dev/null || true
fi
printf '\n=== CAPACIDADE PARA ESTUDO ===\n'
mem_kib=$(awk '/^MemTotal:/{print $2}' /proc/meminfo)
cpus=$(getconf _NPROCESSORS_ONLN)
if [[ "$mem_kib" =~ ^[0-9]+$ ]] && ((mem_kib >= 4*1024*1024));then echo 'RAM >= 4 GiB: sim'
else echo 'RAM >= 4 GiB: não (planejar limites para serviços)';fi
if ((cpus>=2));then echo 'CPU >= 2 lógicas: sim'
else echo 'CPU >= 2 lógicas: não';fi
printf '\nNenhuma configuração ou serviço foi alterado.\n'
