FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o tams-web-ui main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/tams-web-ui .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static

EXPOSE 8090

CMD ["./tams-web-ui"]