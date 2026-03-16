# Build stage
FROM golang:1.21 AS builder

WORKDIR /build

RUN apt-get update && apt-get install -y git make && rm -rf /var/lib/apt/lists/*

# 使用已提交的 vendor 构建，无需外网拉依赖
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -a -installsuffix cgo -o istio-config-exporter .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /root

COPY --from=builder /build/istio-config-exporter .

EXPOSE 9102

ENTRYPOINT ["./istio-config-exporter"]
