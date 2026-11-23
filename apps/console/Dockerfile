FROM golang:1.27-trixie AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY internal/env/ ./internal/env/
COPY internal/httpapp/ ./internal/httpapp/
COPY apps/console/ ./apps/console/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/console ./apps/console/cmd/console

FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /app
COPY --from=build /out/console /app/console
EXPOSE 8092
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=3 CMD ["/app/console", "saude", "-url", "http://127.0.0.1:8092/readyz"]
ENTRYPOINT ["/app/console"]
CMD ["-addr", "0.0.0.0:8092"]
