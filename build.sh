#!/usr/bin/env bash
# Compila e testa o go-concierge sem Go local: tudo roda no contêiner golang:1.26-alpine,
# com o cache de módulos compartilhado com o sl-concierge.
#
#   ./build.sh            # gofmt + vet + test + binários em bin/ (Windows e Linux)
#   ./build.sh --test     # só vet + test
#   ./build.sh --run      # compila e sobe o okf-admin no Windows (porta 8090) abrindo o navegador
set -euo pipefail
cd "$(dirname "$0")"

command -v docker >/dev/null 2>&1 || { echo "Docker não encontrado" >&2; exit 1; }

GO="MSYS_NO_PATHCONV=1 docker run --rm -v $PWD:/src -w /src -v sl-concierge-gomod:/go/pkg/mod -v sl-concierge-gocache:/root/.cache/go-build golang:1.26-alpine"

case "${1:-}" in
  --test)
    eval "$GO" sh -c "'gofmt -l . && go vet ./... && go test ./...'"
    ;;
  --run)
    eval "$GO" sh -c "'go vet ./... && mkdir -p bin && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -o bin/okf-admin.exe ./cmd/okf-admin'"
    exec ./bin/okf-admin.exe -root ./okf -addr 127.0.0.1:8090 -open
    ;;
  *)
    eval "$GO" sh -c "'gofmt -w . && go mod tidy && go vet ./... && go test ./... && mkdir -p bin && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags=\"-s -w\" -o bin/okf-admin.exe ./cmd/okf-admin && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags=\"-s -w\" -o bin/okf-admin ./cmd/okf-admin'"
    echo "binários em bin/: okf-admin.exe (Windows) e okf-admin (Linux)"
    ;;
esac
