# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.25

########################
# 1. Stage: build
########################
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-bookworm AS build

ARG TARGETOS
ARG TARGETARCH

ENV CGO_ENABLED=0 \
    GODEBUG=asyncpreemptoff=1

WORKDIR /app

# Сначала модули — кешируем go mod download отдельным слоем + cache mount
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Затем весь код
COPY . .

# Собираем сервис и healthcheck-пробник (для distroless HEALTHCHECK).
# cache mounts переживают ребилды и не попадают в финальный образ.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/ai-worker . && \
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
      org.opencontainers.image.title="incidentgpt-ai-worker" \
      org.opencontainers.image.description="AI worker for IncidentGPT (OpenRouter/OpenAI integration)" \
      org.opencontainers.image.licenses="MIT"

WORKDIR /app

COPY --from=build /out/ai-worker /app/ai-worker
COPY --from=build /out/healthcheck /app/healthcheck

USER nonroot:nonroot

EXPOSE 8080

# distroless без shell → exec-форма + отдельный бинарь.
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/healthcheck", "http://127.0.0.1:8080/healthz"]

ENTRYPOINT ["/app/ai-worker"]
