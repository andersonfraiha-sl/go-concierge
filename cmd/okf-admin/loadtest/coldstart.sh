#!/bin/sh
# coldstart.sh: mede o tempo do binário até responder 200 em /readyz. Uso:
#   DATABASE_URL=... AMBIENTE=dev AUTH_MODE=dev ./coldstart.sh /caminho/okf-admin -root ./okf -addr 127.0.0.1:18093
set -eu
BIN="$1"; shift
START=$(date +%s%N)
"$BIN" "$@" > /tmp/coldstart.log 2>&1 &
PID=$!
for i in $(seq 1 200); do
  CODE=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:18093/readyz" 2>/dev/null || true)
  if [ "$CODE" = "200" ]; then
    END=$(date +%s%N)
    echo "cold start: $(( (END - START) / 1000000 )) ms"
    kill -9 "$PID" 2>/dev/null || true
    exit 0
  fi
  sleep 0.01
done
echo "timeout esperando /readyz"
kill -9 "$PID" 2>/dev/null || true
exit 1
