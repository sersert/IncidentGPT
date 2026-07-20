# syntax=docker/dockerfile:1.6

ARG GO_VERSION=1.24

########################
# 1. Stage: build
########################
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-bookworm AS build

# Эти аргументы будет прокидывать buildx при --platform
ARG TARGETOS
ARG TARGETARCH

# Немного тюнинга рантайма Go:
# - CGO_ENABLED=0 — статичный бинарь, проще для distroless.
# - GODEBUG=asyncpreemptoff=1 — обход бага с netpoll/epoll под buildx/qemu.
ENV CGO_ENABLED=0 \
    GODEBUG=asyncpreemptoff=1

WORKDIR /app

# Сначала только модули — чтобы кешировать go mod download
COPY go.mod go.sum ./
RUN go mod download

# Потом весь остальной код
COPY . .

# Собираем под ту платформу, которую передал buildx (linux/amd64 и т.п.)
RUN GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o enricher .

########################
# 2. Stage: runtime
########################
FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /app

# Кладём бинарь из builder-стейджа
COPY --from=build /app/enricher /app/enricher

# Небольшой sanity: не root
USER nonroot:nonroot

EXPOSE 9099

ENTRYPOINT ["/app/enricher"]
