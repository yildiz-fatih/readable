# build stage
FROM golang:1.27.1-alpine3.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/readableapi ./cmd/api

# runtime stage
FROM alpine:3.24.1

WORKDIR /app

COPY --from=builder /out/readableapi ./readableapi

ENTRYPOINT ["./readableapi"]
