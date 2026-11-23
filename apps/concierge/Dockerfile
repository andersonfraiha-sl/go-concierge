FROM golang:1.27-trixie AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY internal/ ./internal/
COPY apps/concierge/ ./apps/concierge/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/concierge ./apps/concierge/cmd/concierge \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/okf-run ./apps/concierge/cmd/okf-run

FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /app
COPY --from=build /out/ /app/
COPY --chown=nonroot:nonroot okf /app/okf
EXPOSE 8090
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=3 CMD ["/app/concierge", "saude"]
ENTRYPOINT ["/app/concierge"]
CMD ["-root", "/app/okf", "-addr", "0.0.0.0:8090", "-env", "/app/.env"]
