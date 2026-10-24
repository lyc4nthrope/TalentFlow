FROM node:22-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
COPY . .

RUN npm ci --only=production

FROM node:22-alpine AS runner

WORKDIR /app

COPY --from=builder /app/package.json ./
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app ./

WORKDIR /app/microservicios/gestion-empleados

EXPOSE 8081

CMD ["npm", "start"]