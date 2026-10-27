FROM node:22-alpine AS builder

WORKDIR /app

# Usar 'package*.json' permite copiar package.json y package-lock.json (si existe) sin fallar
COPY package*.json ./

COPY . .

# 'npm install' no exige obligatoriamente un package-lock.json
RUN npm install --only=production

FROM node:22-alpine AS runner

WORKDIR /app

COPY --from=builder /app ./

EXPOSE 8081

CMD ["npm", "start"]