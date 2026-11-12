# Etapas (proyecto final, F1): base -> test -> reportes (solo para `make test`) y runner,
# la última, que es la que construyen `docker build` y docker compose: sin vendor ni pruebas.
FROM php:8.2-cli-alpine AS base

RUN docker-php-ext-install pdo pdo_mysql

WORKDIR /app

# --- Pruebas: PHPUnit con cobertura (pcov) sobre SQLite en memoria ---
FROM base AS test

RUN apk add --no-cache --virtual .deps-compilacion $PHPIZE_DEPS \
    && pecl install pcov \
    && docker-php-ext-enable pcov \
    && apk del .deps-compilacion

COPY --from=composer:2.8 /usr/bin/composer /usr/bin/composer

COPY composer.json composer.lock ./
RUN composer install --no-interaction --no-progress --prefer-dist

COPY phpunit.xml init.sql ./
COPY src ./src
COPY public ./public
COPY tests ./tests

# El código de salida queda en reports/estado y no corta el build: así los reportes
# (JUnit incluido) salen también cuando una prueba falla. `make test` lo revisa.
RUN mkdir -p reports \
    && (vendor/bin/phpunit && php tests/cobertura-minima.php reports/clover.xml 70; echo $? > reports/estado)

FROM scratch AS reportes
COPY --from=test /app/reports /

# --- Imagen de ejecución ---
FROM base AS runner

COPY src ./src
COPY public ./public

EXPOSE 8082

# -q: sin las líneas "Accepted/Closing" del servidor; cada petición deja su log JSON (Reto 8).
CMD ["php", "-S", "0.0.0.0:8082", "-q", "-t", "public", "public/index.php"]
