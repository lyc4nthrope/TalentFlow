FROM php:8.2-cli-alpine AS runner

RUN docker-php-ext-install pdo pdo_mysql

WORKDIR /app

COPY . .

EXPOSE 8082

# -q: sin las líneas "Accepted/Closing" del servidor; cada petición deja su log JSON (Reto 8).
CMD ["php", "-S", "0.0.0.0:8082", "-q", "-t", "public", "public/index.php"]