FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install git for module downloads when needed.
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main .

FROM alpine:latest AS runner

WORKDIR /app

# Add CA certs for outbound HTTPS calls.
RUN apk add --no-cache ca-certificates

COPY --from=builder /app/main /app/main

EXPOSE 5000

ENTRYPOINT ["/app/main"]
