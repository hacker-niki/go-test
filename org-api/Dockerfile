FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download && go mod verify

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/api

FROM alpine:3.19

RUN addgroup -S app && adduser -S app -G app
WORKDIR /app

COPY --from=builder /app/server .

USER app

EXPOSE 8080
CMD ["/app/server"]
