FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/perfiles-service .

FROM alpine:3.20 AS runner

RUN adduser -D -u 10001 app
USER app
WORKDIR /app
COPY --from=builder /out/perfiles-service /app/perfiles-service

EXPOSE 8083

CMD ["/app/perfiles-service"]
