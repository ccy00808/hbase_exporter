# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-X github.com/prometheus/common/version.Version=${VERSION} \
              -X github.com/prometheus/common/version.Revision=${COMMIT}" \
    -o /hbase_exporter .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 1000 exporter
USER exporter
COPY --from=builder /hbase_exporter /usr/local/bin/hbase_exporter
EXPOSE 9115
ENTRYPOINT ["/usr/local/bin/hbase_exporter"]
