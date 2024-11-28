FROM golang:1.21-alpine3.20 AS builder

WORKDIR /app

ADD go.* ./
RUN go mod download

ADD . .
RUN go build -o nox-lobby ./cmd/nox-lobby

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
COPY --from=builder /app/nox-lobby /usr/bin/nox-lobby

EXPOSE 80
EXPOSE 6060

ENTRYPOINT ["nox-lobby", "serve", "--host=:80", "--monitor=:6060"]