#!/usr/bin/env bash
# MackNa Node 02 sizing report for old x86-64 all-in-one hardware.
# Read only: never changes CPU, memory, Docker, disks or system services.
set -Eeuo pipefail
[[ "${1:-}" == "--report" && $# == 1 ]] || {
  echo "Uso: bash scripts/node02-capacity-report.sh --report" >&2
  exit 2
}
printf '=== MACKNA NODE 02 - PERFIL DE CAPACIDADE (somente leitura) ===\n'
printf 'Arquitetura: %s\n' "$(uname -m)"
if command -v lscpu >/dev/null 2>&1; then
  lscpu | grep -E '^(Model name|CPU\(s\)|Core\(s\) per socket|Socket\(s\)|Thread\(s\) per core|Architecture):' || true
fi
mem_kib=$(awk '/^MemTotal:/{print $2}' /proc/meminfo)
avail_kib=$(awk '/^MemAvailable:/{print $2}' /proc/meminfo)
printf 'RAM física: %s MiB | disponível agora: %s MiB\n' "$((mem_kib/1024))" "$((avail_kib/1024))"
if command -v free >/dev/null; then free -h; fi
printf '\n=== ARMAZENAMENTO ===\n'
df -hT /
if command -v lsblk >/dev/null; then
  lsblk -o NAME,SIZE,ROTA,TYPE,FSTYPE,MOUNTPOINTS 2>/dev/null || true
fi
available_disk_kib=$(df -Pk / | awk 'NR==2{print $4}')
printf 'Livre na raiz: %s GiB\n' "$((available_disk_kib/1024/1024))"
printf '\n=== CPU E CRIPTOGRAFIA ===\n'
if grep -qiE '(^|[[:space:]])aes([[:space:]]|$)' /proc/cpuinfo; then
  echo 'AES-NI: disponível'
else
  echo 'AES-NI: ausente/desconhecido (criptografia AES funcionará por software, mais lenta)'
fi
if grep -qiE '(^|[[:space:]])avx2([[:space:]]|$)' /proc/cpuinfo; then
  echo 'AVX2: disponível'
else
  echo 'AVX2: ausente/desconhecido (validar requisitos de imagens de terceiros)'
fi
printf '\n=== RECOMENDAÇÕES PARA PERFIL PEQUENO ===\n'
echo 'Concorrência de builds de terceiros: no máximo 1, sempre isolado'
echo 'Réplicas iniciais por serviço: 1; evitar múltiplos bancos pesados'
echo 'Reservar no mínimo 15 a 20 GiB livres no SSD; backups externos obrigatórios'
echo 'Aplicar limites reais de memória/CPU/disco a cada projeto antes de habilitar deploys'
if ((mem_kib < 6*1024*1024)); then
  echo 'ALERTA: menos de 6 GiB RAM instalados; restringir a monitoramento/laboratório'
elif ((mem_kib < 12*1024*1024)); then
  echo 'PERFIL: compacto (6-12 GiB); habilitação de deploys exige quotas e homologação'
else
  echo 'PERFIL: capacidade acima de 12 GiB; quotas ainda obrigatórias'
fi
if ((available_disk_kib < 20*1024*1024)); then
  echo 'ALERTA: menos de 20 GiB livres; não iniciar builds/volumes novos'
fi
echo 'Este relatório não habilita deploys, não valida SSD/temperatura e não certifica produção.'
