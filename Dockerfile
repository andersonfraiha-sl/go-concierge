# go-concierge: okf-admin (Studio, Console e o canal ATENDE) e okf-run.
# O Studio é compilado no estágio front e embutido no binário pelo go:embed.
FROM node:26-alpine AS front
WORKDIR /src/studio
COPY studio/package.json studio/package-lock.json ./
RUN npm ci
COPY studio/ ./
RUN npm run build

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=front /src/web/studio/dist ./web/studio/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/okf-admin ./cmd/okf-admin \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/okf-run ./cmd/okf-run

FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/ /app/
# O catálogo (agentes, instantâneo do Nexus, var/) vai junto na imagem; no compose ele
# é montado de ./okf para editar pelo Studio sem rebuild.
COPY okf /app/okf
RUN chown -R app /app
USER app
EXPOSE 8090
ENTRYPOINT ["/app/okf-admin"]
CMD ["-root", "/app/okf", "-addr", "0.0.0.0:8090", "-env", "/app/.env"]
