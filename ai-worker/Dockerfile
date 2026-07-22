# syntax=docker/dockerfile:1.6

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

# Копируем модули для кеширования
COPY go.mod go.sum ./
RUN go mod download

# Затем весь код
COPY . .

RUN GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o ai-worker .

########################
# 2. Stage: runtime
########################
FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /app

COPY --from=build /app/ai-worker /app/ai-worker

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/app/ai-worker"]
