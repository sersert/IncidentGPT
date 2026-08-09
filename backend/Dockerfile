FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
# Зависимости ставим отдельным слоем: он не пересобирается при правке кода.
RUN go mod download
# Копируем все исходники, а не один main.go: файлов в пакете стало больше.
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server .

FROM scratch

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.source="https://github.com/sersert/IncidentGPT" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.title="incidentgpt-backend" \
      org.opencontainers.image.description="Incident store and API for IncidentGPT web UI" \
      org.opencontainers.image.licenses="MIT"

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /app/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
