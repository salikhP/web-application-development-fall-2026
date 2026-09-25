FROM golang:1.26.4-alpine AS builder

WORKDIR /app

COPY go.mod ./

COPY . .

RUN go build -o app .

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/app .

ENV PORT=8080
ENV MESSAGE="Hello Docker!"

EXPOSE 8080

CMD ["./app"]
