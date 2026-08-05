# syntax=docker/dockerfile:1.7

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

# Сначала только модули — кешируем go mod download отдельным слоем + cache mount
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Потом весь остальной код
COPY . .

# Собираем сервис и healthcheck-пробник под платформу buildx (linux/amd64 и т.п.).
# cache mounts переживают ребилды и не попадают в финальный образ.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/enricher . && \
    GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/healthcheck ./healthcheck

########################
# 2. Stage: runtime
########################
FROM gcr.io/distroless/base-debian12:nonroot

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.source="https://github.com/sersert/IncidentGPT" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.title="incidentgpt-enricher" \
      org.opencontainers.image.description="Data Enricher for IncidentGPT (Alertmanager -> Prometheus -> Backend/OpenRouter)" \
      org.opencontainers.image.licenses="MIT"

WORKDIR /app

# Кладём бинари из builder-стейджа
COPY --from=build /out/enricher /app/enricher
COPY --from=build /out/healthcheck /app/healthcheck

# Небольшой sanity: не root
USER nonroot:nonroot

EXPOSE 9099

# distroless без shell → exec-форма + отдельный бинарь.
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/healthcheck", "http://127.0.0.1:9099/healthz"]

ENTRYPOINT ["/app/enricher"]
