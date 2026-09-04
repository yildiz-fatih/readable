# build stage
FROM golang:1.27.1-alpine3.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/epubservice .

# runtime stage
FROM pandoc/core:3.10.0.0

WORKDIR /app

COPY --from=builder /out/epubservice ./epubservice

ENTRYPOINT ["./epubservice"]
