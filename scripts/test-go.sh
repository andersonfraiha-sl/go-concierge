#!/usr/bin/env bash
set -u

modo=${1:-focado}
if (($# > 0)); then shift; fi

case "$modo" in
  focado)
    pacote=${1:-./internal/atende}
    regex=${2:-^TestV2}
    if (($# > 2)) || [[ -z "$pacote" || -z "$regex" || "$pacote" == *...* ]]; then
      printf 'Uso: %s focado [pacote [regex]] (use completo para ./...)\n' "$0" >&2
      exit 2
    fi
    comando=(go test "$pacote" -run "$regex")
    ;;
  integracao)
    if (($# != 2)) || [[ -z "$1" || -z "$2" || "$1" == *...* ]]; then
      printf 'Uso: %s integracao pacote regex (use completo para ./...)\n' "$0" >&2
      exit 2
    fi
    comando=(go test "$1" -run "$2")
    ;;
  completo)
    if (($# != 0)); then
      printf 'Uso: %s completo\n' "$0" >&2
      exit 2
    fi
    comando=(go test ./...)
    ;;
  *)
    printf 'Modo desconhecido: %s\n' "$modo" >&2
    exit 2
    ;;
esac
dir_logs=${LOG_DIR:-${TMPDIR:-/tmp}}
mkdir -p "$dir_logs"
log="$dir_logs/go-concierge-${modo}-$(date +%Y%m%dT%H%M%S)-$$.log"

if TZ=UTC "${comando[@]}" >"$log" 2>&1; then
  codigo=0
else
  codigo=$?
fi
if ((codigo == 0)); then
  awk '/^(ok[[:space:]]|\?[[:space:]]|PASS$)/' "$log" | awk 'NR <= 78 { print } END { if (NR > 78) printf "... %d linhas omitidas; consulte o log completo.\n", NR - 78 }'
else
  linhas=$(awk '/^(ok[[:space:]]|\?[[:space:]]|PASS$|FAIL([[:space:]]|$)|--- FAIL:|# )/ || /:[0-9]+:/ || /(^|[[:space:]])(error|Error|panic:)/' "$log" | tail -n 79)
  if [[ -n "$linhas" ]]; then
    printf '%s\n' "$linhas"
  else
    tail -n 79 "$log"
  fi
fi
printf 'Log completo: %s (exit code %s)\n' "$log" "$codigo"
exit "$codigo"
