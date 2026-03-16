# Build stage
FROM golang:1.21 AS builder

WORKDIR /build

# No git/make needed for -mod=vendor build

# 使用已提交的 vendor 构建，无需外网拉依赖
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -a -installsuffix cgo -o istio-config-exporter .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /build/istio-config-exporter .
# Alpine 已有 nobody (UID 65534)，直接使用
RUN chown -R nobody:nobody /app

USER nobody

EXPOSE 9102

ENTRYPOINT ["./istio-config-exporter"]
