FROM composer:2 AS builder

WORKDIR /app

COPY package.json ./
COPY composer.json composer.lock% ./

RUN composer install --no-dev --no-scripts --prefer-dist --optimize-autoloader

COPY microservicios/gestion-departamentos ./microservicios/gestion-departamentos

FROM php:8.2-apache AS runner

WORKDIR /app

RUN docker-php-ext-install pdo pdo_mysql pdo_pgsql

COPY --from=builder /app /app

EXPOSE 8081

CMD ["php", "-S", "0.0.0.0:8081", "-t", "public"]