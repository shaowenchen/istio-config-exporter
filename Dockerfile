# Build stage
FROM golang:1.21 AS builder

WORKDIR /build

RUN apt-get update && apt-get install -y git make && rm -rf /var/lib/apt/lists/*

# Copy go.mod first so dependency download is cached when code changes (go.sum may be missing in repo)
COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -a -installsuffix cgo -o istio-config-exporter .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /root

COPY --from=builder /build/istio-config-exporter .

EXPOSE 9102

ENTRYPOINT ["./istio-config-exporter"]
