# Etapa 1: compilación. Solo existe durante el build; no llega a la imagen final.
FROM golang:1.25-alpine AS builder
WORKDIR /src

# Dependencias primero: si solo cambia el código, Docker reutiliza esta capa.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Binario estático (sin libc) y sin información de depuración: más pequeño.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /notificaciones ./cmd/notificaciones

# Etapa 2: imagen final mínima. distroless/static no tiene shell ni gestor de
# paquetes (menos superficie de ataque) y corre como usuario sin privilegios.
FROM gcr.io/distroless/static-debian12:nonroot AS runner
COPY --from=builder /notificaciones /notificaciones
USER nonroot:nonroot
EXPOSE 8084
ENTRYPOINT ["/notificaciones"]
