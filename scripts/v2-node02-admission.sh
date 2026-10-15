#!/usr/bin/env bash
# MackNa v2 Node 02: fail-closed, read-only admission check for a NEW server.
# Does NOT install packages, modify Docker, open ports, or start services.
set -Eeuo pipefail
[[ "${1:-}" == "--check" && $# == 1 ]] || {
 echo "Uso: bash scripts/v2-node02-admission.sh --check" >&2;exit 2;
}
printf '=== MackNa v2 / Node 02: ADMISSÃO DE HOST ===\n'
fail=0
arch=$(uname -m)
printf 'Arquitetura: %s\n' "$arch"
if [[ "$arch" != x86_64 && "$arch" != aarch64 ]];then
 echo 'BLOQUEADO: binários não disponíveis para esta arquitetura';fail=1
fi
if [[ -r /etc/os-release ]];then
 . /etc/os-release
 printf 'Distribuição: %s\n' "${PRETTY_NAME:-unknown}"
 case "${ID:-}" in debian|ubuntu) :;; *) echo 'BLOQUEADO: SO ainda não validado';fail=1;; esac
else
 echo 'BLOQUEADO: /etc/os-release não disponível';fail=1
fi
if [[ ! -d /run/systemd/system ]];then
 echo 'BLOQUEADO: systemd não disponível';fail=1
fi
for svc in mackna-agent mackna-gate mackna-deployer mackna-worker;do
 if command -v systemctl >/dev/null && systemctl is-active --quiet "$svc" 2>/dev/null;then
  echo "BLOQUEADO: serviço preexistente $svc";fail=1
 fi
done
if command -v docker >/dev/null;then
 if docker info >/dev/null 2>&1;then
  names=$(docker ps -a --format '{{.Names}}') || { echo 'BLOQUEADO: falha ao inspecionar contêineres'; fail=1; names=''; }
  if [[ -n "$names" ]];then
   echo 'BLOQUEADO: host não está vazio; preserve os serviços existentes e revise a instalação manualmente'
   printf '%s\n' "$names" | sed -n '1,25p'
   fail=1
  fi
 else
  echo 'BLOQUEADO: Docker instalado porém não responde; não assumir host vazio';fail=1
 fi
else
 echo 'Docker ainda não instalado: permitido para um nó novo (instalação futura separada)'
fi
if ! command -v ss >/dev/null;then
 echo 'BLOQUEADO: ss não disponível para verificar portas';fail=1
else
 for port in 80 443 8090 8091 8093;do
  sockets=$(ss -H -ltn "( sport = :$port )" 2>/dev/null) || {
   echo "BLOQUEADO: não foi possível inspecionar a porta $port"
   fail=1
   continue
  }
  if [[ -n "$sockets" ]];then
   echo "BLOQUEADO: porta TCP $port já em uso";fail=1
  fi
 done
fi
memory_kib=$(awk '/^MemTotal:/{print $2}' /proc/meminfo)
cpus=$(getconf _NPROCESSORS_ONLN)
disk_avail_kib=$(df -Pk / | awk 'NR==2{print $4}')
printf 'CPU lógicas: %s | MemTotal: %s KiB | Livre na raiz: %s KiB\n' "$cpus" "$memory_kib" "$disk_avail_kib"
if [[ ! "$cpus" =~ ^[0-9]+$ || "$cpus" -lt 2 ]];then echo 'BLOQUEADO: CPU insuficiente ou ilegível';fail=1;fi
if [[ ! "$memory_kib" =~ ^[0-9]+$ ]] || ((memory_kib<4*1024*1024));then echo 'BLOQUEADO: menos de 4 GiB RAM';fail=1;fi
if [[ ! "$disk_avail_kib" =~ ^[0-9]+$ ]] || ((disk_avail_kib<20*1024*1024));then echo 'BLOQUEADO: menos de 20 GiB livres na raiz';fail=1;fi
if ((fail));then
 echo 'RESULTADO: BLOQUEADO. Nenhuma alteração realizada.'
 exit 12
fi
echo 'RESULTADO: PRÉ-REQUISITOS BÁSICOS CONFIRMADOS.'
echo 'IMPORTANTE: este teste não instala MackNa nem certifica segurança/produção.'
echo 'Antes de provisionar, confirmar modelo de CPU, SSD, rede, backup externo e UPS.'
