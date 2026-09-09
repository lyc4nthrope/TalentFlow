FROM node:20-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
COPY shared/package*.json ./shared/
COPY microservicios/gestion-empleados ./microservicios/gestion-empleados

RUN npm ci

COPY shared ./shared
COPY microservicios/gestion-empleados ./microservicios/gestion-empleados

FROM node:20-alpine AS runner

WORKDIR /app

COPY --from=builder /app/package.json /app/package-lock.json ./
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/shared ./shared
COPY --from=builder /app/microservicios/gestion-empleados ./microservicios/gestion-empleados

WORKDIR /app/microservicios/gestion-empleados

EXPOSE 8080

CMD ["npm", "start"]