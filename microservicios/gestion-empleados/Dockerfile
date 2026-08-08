FROM node:20-alpine

WORKDIR /app

COPY package.json package-lock.json ./
COPY shared ./shared
COPY microservicios/gestion-empleados ./microservicios/gestion-empleados

RUN npm ci

WORKDIR /app/microservicios/gestion-empleados

EXPOSE 8080

CMD ["npm", "start"]
