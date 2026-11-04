# go-concierge: okf-admin (Studio, Console e o canal ATENDE) e okf-run.
# O Studio é compilado no estágio front e embutido no binário pelo go:embed.
FROM node:26.13-trixie-slim AS front
WORKDIR /src/web/studio
# Node 26 LTS (Debian trixie); instala o pnpm pinado em package.json (packageManager).
RUN npm install -g pnpm@12.16.1
COPY web/studio/package.json web/studio/pnpm-lock.yaml web/studio/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/studio/ ./
RUN pnpm run build

FROM golang:1.27-trixie AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=front /src/web/studio/dist ./web/studio/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/okf-admin ./cmd/okf-admin \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/okf-run ./cmd/okf-run

# distroless: sem shell nem gerenciador de pacotes; já traz ca-certificates e tzdata.
FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /app
COPY --from=build /out/ /app/
# O catálogo (agentes, instantâneo do Nexus, var/) vai junto na imagem; no compose ele
# é montado de ./okf para editar pelo Studio sem rebuild.
COPY --chown=nonroot:nonroot okf /app/okf
EXPOSE 8090
# forma exec: a imagem não tem shell; o binário consulta o /readyz (subcomando saude)
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=3 CMD ["/app/okf-admin", "saude"]
ENTRYPOINT ["/app/okf-admin"]
CMD ["-root", "/app/okf", "-addr", "0.0.0.0:8090", "-env", "/app/.env"]
