FROM node:20-alpine
WORKDIR /usr/src/app
ENV NODE_ENV=production
# package-lock.json propio del Gateway: instalación reproducible (mismas versiones en cada build).
COPY package.json package-lock.json ./
RUN npm ci --omit=dev
COPY . .
EXPOSE 8080
CMD ["npm","start"]
