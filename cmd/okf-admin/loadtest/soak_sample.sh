#!/bin/sh
# soak_sample.sh: amostra memória do processo (tasklist, Windows) e conexões ativas no
# Postgres durante o soak. Sem pprof/expvar exposto no binário (ver relatório), não há como
# medir goroutines por fora; a amostra fica em memória RSS + conexões do pool.
PID="$1"
OUT="$2"
echo "timestamp_s,mem_kb,pg_conexoes" > "$OUT"
START=$(date +%s)
while true; do
  NOW=$(date +%s)
  MEM=$(tasklist //FI "PID eq $PID" //FO CSV //NH 2>/dev/null | tr -d '"' | awk -F',' '{gsub(/[^0-9]/,"",$5); print $5}')
  CONN=$(docker exec gc-perf-pg-desres psql -U concierge -d concierge -t -c "SELECT count(*) FROM pg_stat_activity WHERE datname='concierge';" 2>/dev/null | tr -d ' ')
  echo "$((NOW - START)),$MEM,$CONN" >> "$OUT"
  sleep 15
done
