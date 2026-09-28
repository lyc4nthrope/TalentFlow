# Etapa 1: compila y empaqueta con Maven. No llega a la imagen final.
FROM maven:3.9-eclipse-temurin-17-alpine AS builder
WORKDIR /build
# Dependencias primero: si solo cambia el código, Docker reutiliza esta capa (no re-descarga).
COPY pom.xml .
RUN mvn -q -B dependency:go-offline
COPY src ./src
# Las pruebas se ejecutan fuera del build de la imagen (mvn test / CI).
RUN mvn -q -B package -DskipTests

# Etapa 2: solo el JRE (sin compilador ni Maven) y el jar.
FROM eclipse-temurin:17-jre-alpine AS runner
WORKDIR /app
# Usuario sin privilegios.
RUN addgroup -S vacaciones && adduser -S -G vacaciones vacaciones
COPY --from=builder /build/target/gestion-vacaciones.jar app.jar
USER vacaciones
EXPOSE 8085
# La JVM respeta el límite de memoria del contenedor (75 % para el heap).
ENTRYPOINT ["java", "-XX:MaxRAMPercentage=75", "-jar", "app.jar"]
