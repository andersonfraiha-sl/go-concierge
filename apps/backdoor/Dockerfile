FROM node:26.13-trixie-slim AS front
WORKDIR /src/apps/backdoor/web/studio
ENV HUSKY=0
RUN npm install -g pnpm@12.16.1
COPY apps/backdoor/web/studio/package.json apps/backdoor/web/studio/pnpm-lock.yaml apps/backdoor/web/studio/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY apps/backdoor/web/studio/ ./
RUN pnpm run build

FROM golang:1.27-trixie AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY internal/env/ ./internal/env/
COPY internal/httpapp/ ./internal/httpapp/
COPY apps/backdoor/ ./apps/backdoor/
COPY --from=front /src/apps/backdoor/web/studio/dist ./apps/backdoor/web/studio/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/backdoor ./apps/backdoor/cmd/backdoor

FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /app
COPY --from=build /out/backdoor /app/backdoor
EXPOSE 8091
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=3 CMD ["/app/backdoor", "saude", "-url", "http://127.0.0.1:8091/readyz"]
ENTRYPOINT ["/app/backdoor"]
CMD ["-addr", "0.0.0.0:8091"]
