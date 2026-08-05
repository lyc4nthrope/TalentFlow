FROM node:20-alpine

WORKDIR /app

COPY package.json package-lock.json ./
COPY shared ./shared
COPY services/empleados ./services/empleados

RUN npm ci

WORKDIR /app/services/empleados

EXPOSE 8080

CMD ["npm", "start"]
