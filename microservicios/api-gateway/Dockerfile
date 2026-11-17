# Etapa 1: Construcción de la aplicación
FROM node:22-alpine AS builder
WORKDIR /usr/src/app
# package-lock.json propio del Gateway: instalación reproducible (mismas versiones en cada build).
COPY package.json package-lock.json ./
RUN npm ci --omit=dev
# Etapa 2: Imagen final limpia
FROM node:22-alpine AS runner

WORKDIR /usr/src/app
ENV NODE_ENV=production

COPY --from=builder --chown=node:node /usr/src/app/node_modules ./node_modules
COPY --chown=node:node package.json package-lock.json ./
COPY --chown=node:node . .

USER node

EXPOSE 8080
CMD ["npm","start"]
