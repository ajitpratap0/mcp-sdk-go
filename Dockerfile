FROM golang:1.24.2-alpine AS builder

WORKDIR /app

COPY . .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mcp-sdk-go ./mcp.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/mcp-sdk-go .

ENTRYPOINT ["./mcp-sdk-go"] 