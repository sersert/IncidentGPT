<<<<<<< HEAD
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /out/sanitizer ./cmd/sanitizer

FROM gcr.io/distroless/static-debian12:nonroot
USER nonroot:nonroot
COPY --from=build /out/sanitizer /sanitizer
EXPOSE 8080
=======
# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.24

########################
# 1. Stage: build
########################
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-bookworm AS build

ARG TARGETOS
ARG TARGETARCH

ENV CGO_ENABLED=0 \
    GODEBUG=asyncpreemptoff=1

WORKDIR /src

# У sanitizer нет внешних зависимостей (stdlib only) → go.sum отсутствует.
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
COPY healthcheck ./healthcheck

# Собираем сервис и healthcheck-пробник под платформу buildx.
# cache mounts переживают ребилды и не попадают в финальный образ.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/sanitizer ./cmd/sanitizer && \
    GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/healthcheck ./healthcheck

########################
# 2. Stage: runtime
########################
FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.source="https://github.com/sersert/IncidentGPT" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.title="incidentgpt-sanitizer" \
      org.opencontainers.image.description="Data Sanitizer security gateway for IncidentGPT" \
      org.opencontainers.image.licenses="MIT"

USER nonroot:nonroot

COPY --from=build /out/sanitizer /sanitizer
COPY --from=build /out/healthcheck /healthcheck

EXPOSE 8080

# distroless без shell → exec-форма + отдельный бинарь.
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/healthcheck", "http://127.0.0.1:8080/healthz"]

>>>>>>> refs/remotes/origin/main
ENTRYPOINT ["/sanitizer"]
