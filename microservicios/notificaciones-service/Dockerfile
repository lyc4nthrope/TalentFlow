FROM maven:3.9-eclipse-temurin-21 AS builder

WORKDIR /build
COPY pom.xml ./
RUN mvn -q -B dependency:go-offline
COPY src ./src
RUN mvn -q -B -Dmaven.test.skip=true package

FROM eclipse-temurin:21-jre-alpine AS runner

RUN adduser -D -u 10001 app
USER app
WORKDIR /app
COPY --from=builder /build/target/notificaciones-service.jar /app/app.jar

EXPOSE 8084

ENTRYPOINT ["java", "-XX:MaxRAMPercentage=75", "-jar", "/app/app.jar"]
