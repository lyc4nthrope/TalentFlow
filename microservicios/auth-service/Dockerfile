FROM golang:1.27.1-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /auth-service ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot AS runner
WORKDIR /app
COPY --from=build /auth-service /auth-service
EXPOSE 8086
ENTRYPOINT ["/auth-service"]