FROM golang:1.26.8 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION}" -o /out/fortiadc-exporter ./cmd/fortiadc-exporter

FROM debian:trixie-20260918-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /out/fortiadc-exporter /usr/local/bin/fortiadc-exporter
USER nobody
EXPOSE 9634
ENTRYPOINT ["/usr/local/bin/fortiadc-exporter"]
CMD ["-config", "/etc/fortiadc-exporter/config.yml"]
