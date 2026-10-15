#!/usr/bin/env bash
# Controlled loopback-only MackNa load probe. Never load-test IMORTAIS DB.
set -Eeuo pipefail
URL="${1:-http://127.0.0.1:8090/api/v1/health}"
case "$URL" in
 http://127.0.0.1:8090/api/v1/health|http://localhost:8090/api/v1/health) ;;
 *) echo "Only loopback MackNa health is allowed" >&2; exit 2 ;;
esac
DURATION="${DURATION:-60}"
CONCURRENCY="${CONCURRENCY:-2}"
[[ "$DURATION" =~ ^[0-9]+$ ]] && (( DURATION>=10 && DURATION<=300 )) || exit 2
[[ "$CONCURRENCY" =~ ^[0-9]+$ ]] && (( CONCURRENCY>=1 && CONCURRENCY<=4 )) || exit 2
end=$((SECONDS+DURATION))
dir=$(mktemp -d);trap 'rm -rf "$dir"' EXIT
worker(){
 local id="$1" n=0 ok=0 fail=0
 while (( SECONDS < end ));do
  code=$(curl --noproxy '*' -s -o /dev/null -w '%{http_code}' --connect-timeout 2 --max-time 3 "$URL" || true)
  ((n+=1));if [[ "$code" == 200 ]];then ((ok+=1));else ((fail+=1));fi
  sleep 0.25
 done
 printf '%s %s %s\n' "$n" "$ok" "$fail" >"$dir/$id"
}
echo "Probe: $URL, duration=${DURATION}s, workers=${CONCURRENCY}"
echo "Baseline containers:"
docker ps --format 'table {{.Names}}\t{{.Status}}'
echo "Baseline memory:"
free -m
for ((i=1;i<=CONCURRENCY;i++));do worker "$i" & done
wait
awk '{total+=$1;ok+=$2;fail+=$3} END {printf "Requests=%d OK=%d Failed=%d Success=%.2f%%\n",total,ok,fail,total?100*ok/total:0}' "$dir"/*
echo "After containers:"
docker ps --format 'table {{.Names}}\t{{.Status}}'
echo "After memory:"
free -m
