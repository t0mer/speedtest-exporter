FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG VERSION=dev
WORKDIR /build
ENV GOTOOLCHAIN=local
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o speedtest-exporter ./cmd/speedtest-exporter/

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/speedtest-exporter /speedtest-exporter
EXPOSE 9090
VOLUME ["/data"]
ENTRYPOINT ["/speedtest-exporter", "serve"]
