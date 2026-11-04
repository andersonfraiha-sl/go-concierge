#!/usr/bin/env bash
# Compila e testa o go-concierge sem Go nem Node locais: o Studio roda no contêiner
# node:26.13-trixie-slim (testes e build em web/studio/dist, que o Go embute) e o Go no
# golang:1.27-trixie, com o cache de módulos compartilhado com o sl-concierge.
#
#   ./build.sh            # Studio (lint + test + build) + gofmt + vet + test + binários em bin/ (Windows e Linux)
#   ./build.sh --test     # só os testes: Studio (lint + typecheck + test), vet e Go
#   ./build.sh --run      # compila o Studio, sobe o Postgres (compose) e roda o okf-admin no Windows (porta 8090)
#
# Os testes de internal/banco sobem um Postgres por testcontainers: o contêiner de build fala
# com o Docker do host pelo socket e alcança as portas publicadas por host.docker.internal.
set -euo pipefail
cd "$(dirname "$0")"

command -v docker >/dev/null 2>&1 || { echo "Docker não encontrado" >&2; exit 1; }

# node_modules num volume anônimo: o pnpm install do Linux não sobrescreve o node_modules do Windows.
# O Node 26 não traz o corepack: ele vem do npm e instala o pnpm pinado em package.json (packageManager).
PNPM="npm install -g corepack@latest >/dev/null && corepack enable && corepack install && pnpm install --frozen-lockfile"
STUDIO="MSYS_NO_PATHCONV=1 docker run --rm -e HUSKY=0 -v $PWD:/src -v /src/web/studio/node_modules -w /src/web/studio node:26.13-trixie-slim"
DOCKER_DO_HOST="-v /var/run/docker.sock:/var/run/docker.sock -e TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal -e TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock --add-host=host.docker.internal:host-gateway"
GO="MSYS_NO_PATHCONV=1 docker run --rm $DOCKER_DO_HOST -v $PWD:/src -w /src -v sl-concierge-gomod:/go/pkg/mod -v sl-concierge-gocache:/root/.cache/go-build golang:1.27-trixie"

case "${1:-}" in
  --test)
    eval "$STUDIO" sh -c "'$PNPM && pnpm run lint && pnpm run typecheck && pnpm test'"
    eval "$GO" sh -c "'gofmt -l . && go vet ./... && go test -p 1 ./...'"
    ;;
  --run)
    eval "$STUDIO" sh -c "'$PNPM && pnpm run lint && pnpm test && pnpm run build'"
    docker compose up -d --wait postgres
    # o DATABASE_URL do ambiente ou do .env vence; sem nenhum, o Postgres do compose
    if [ -z "${DATABASE_URL:-}" ] && ! grep -qs '^DATABASE_URL=.' .env; then
      export DATABASE_URL="postgres://concierge:concierge@127.0.0.1:${POSTGRES_PORT:-5432}/concierge?sslmode=disable"
    fi
    eval "$GO" sh -c "'go vet ./... && mkdir -p bin && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -o bin/okf-admin.exe ./cmd/okf-admin'"
    exec ./bin/okf-admin.exe -root ./okf -addr 127.0.0.1:8090 -open
    ;;
  *)
    eval "$STUDIO" sh -c "'$PNPM && pnpm run lint && pnpm test && pnpm run build'"
    eval "$GO" sh -c "'gofmt -w . && go mod tidy && go vet ./... && go test -p 1 ./... && mkdir -p bin && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags=\"-s -w\" -o bin/okf-admin.exe ./cmd/okf-admin && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags=\"-s -w\" -o bin/okf-run.exe ./cmd/okf-run && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags=\"-s -w\" -o bin/okf-admin ./cmd/okf-admin && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags=\"-s -w\" -o bin/okf-run ./cmd/okf-run'"
    echo "binários em bin/: okf-admin e okf-run (.exe Windows e Linux)"
    ;;
esac
