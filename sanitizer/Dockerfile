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
ENTRYPOINT ["/sanitizer"]
