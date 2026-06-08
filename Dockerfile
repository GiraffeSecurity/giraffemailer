# syntax=docker/dockerfile:1

FROM node:22-alpine AS ui
WORKDIR /ui
RUN corepack enable && corepack prepare pnpm@10 --activate
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
ENV NEXT_OUTPUT=export
RUN pnpm build

FROM golang:1.24-bookworm AS builder
ENV GOTOOLCHAIN=auto
WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends git && rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui /ui/out ./internal/ui/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /giraffemail ./cmd/giraffemail

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget
RUN addgroup -S giraffemail && adduser -S giraffemail -G giraffemail
WORKDIR /app
COPY --from=builder /giraffemail /usr/local/bin/giraffemail
COPY config.docker.yaml /etc/giraffemail/config.yaml
RUN mkdir -p /data && chown giraffemail:giraffemail /data
USER giraffemail
EXPOSE 9191
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:9191/healthz || exit 1
ENTRYPOINT ["giraffemail"]
CMD ["serve", "--config", "/etc/giraffemail/config.yaml"]
