# Suite BDD en un contenedor (Reto 6, D13): se ejecuta sin instalar Node en la máquina
# (lo usará Jenkins en el Reto 7). Misma versión de Node con la que se desarrolló la suite.
FROM node:24.14.0-alpine
WORKDIR /app
# package-lock.json propio: instalación reproducible. Sin NODE_ENV=production, porque
# @cucumber/cucumber es una dependencia de desarrollo.
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund && npm cache clean --force
COPY . .
# Los reportes se escriben en /app/reports, montado como volumen desde el host. Se corre
# sin root para que los archivos queden del usuario del host (el Compose fija el uid/gid).
# La caché y los logs de npm van a /tmp, escribible con cualquier uid.
ENV NPM_CONFIG_CACHE=/tmp/.npm NPM_CONFIG_UPDATE_NOTIFIER=false
USER node
CMD ["npm", "test"]
