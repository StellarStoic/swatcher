FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/s-watcher ./cmd/s-watcher

FROM alpine:3.22
RUN apk add --no-cache su-exec \
    && addgroup -S swatcher \
    && adduser -S -G swatcher swatcher
COPY --from=builder /out/s-watcher /usr/local/bin/s-watcher
USER swatcher
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/s-watcher"]
