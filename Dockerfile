# Build stage
FROM golang:1.21 AS builder

WORKDIR /build

# No git/make needed for -mod=vendor build

# 使用已提交的 vendor 构建，无需外网拉依赖
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -a -installsuffix cgo -o istio-config-exporter .

FROM ubuntu:22.04

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /build/istio-config-exporter .
# Ubuntu 已有 nobody (UID 65534)
RUN chown -R 65534:65534 /app

USER 65534

EXPOSE 9102

ENTRYPOINT ["./istio-config-exporter"]
