# Reto 8 — Arquitectura de observabilidad

Diagrama previo a la implementación (paso 1 del PDF). Responde las cuatro preguntas que pide: qué componentes se levantan, cómo llega Prometheus a cada servicio, cómo viajan las trazas y cómo Grafana consume los datos. Las decisiones están en [`PLAN.md`](PLAN.md).

```mermaid
flowchart LR
    cliente["Cliente HTTP"] -->|"traceparent (W3C)"| gw

    subgraph red["Red Docker: microservices-network"]
        direction LR
        subgraph app["Microservicios instrumentados"]
            gw["api-gateway :8080<br/>Node"]
            emp["empleados-service :8081<br/>Node"]
            dep["departamentos-service :8082<br/>PHP"]
            per["perfiles-service :8083<br/>Python"]
            notif["notificaciones-service :8084<br/>Go"]
            vac["vacaciones-service :8085<br/>Java + scheduler"]
            auth["auth-service :8086<br/>Go"]
        end
        broker[("RabbitMQ<br/>traceparent en las<br/>cabeceras del mensaje")]

        subgraph obs["Stack de observabilidad"]
            prom["Prometheus :9090<br/>(host :9091)"]
            jaeger["Jaeger :16686<br/>OTLP :4318"]
            loki["Loki :3100"]
            promtail["Promtail"]
            grafana["Grafana :3000"]
            mailhog["Mailhog :8025"]
        end
    end

    gw --> emp --> dep
    emp -->|"empleado.creado"| broker
    vac -->|"vacaciones.iniciadas<br/>(span raíz del scheduler)"| broker
    broker --> auth & per & notif & vac
    auth -->|"usuario.creado / cuenta.*"| broker

    prom -. "1 · PULL GET /metrics cada 15 s<br/>(por nombre de servicio)" .-> app
    app -- "2 · PUSH spans OTLP/HTTP" --> jaeger
    promtail -. "lee los logs JSON por<br/>el socket de Docker" .-> app
    promtail -- "PUSH logs" --> loki
    grafana -- "3 · PromQL" --> prom
    grafana -- "LogQL" --> loki
    grafana -- "trazas por traceId" --> jaeger
    grafana -- "4 · alerta por email (SMTP)" --> mailhog
```

| Pregunta del PDF | Respuesta |
|---|---|
| 1. ¿Qué componentes se levantan? | Prometheus, Grafana, Loki, Promtail y Jaeger, como servicios del mismo `docker-compose.yml`. Mailhog ya existía (Reto 4) y es el canal de las alertas |
| 2. ¿Cómo se conecta Prometheus a cada servicio? | Está en la misma red `microservices-network` y hace *scraping* (modelo **pull**) a `http://<nombre-del-servicio>:<puerto>/metrics` cada 15 s. Usa los nombres de Docker, nunca IPs ni `localhost`. El Gateway es el primer `scrape_job` |
| 3. ¿Cómo fluyen las trazas? | Cada servicio, con el SDK de OpenTelemetry, **envía** (modelo **push**) sus spans a `http://jaeger:4318` por OTLP. El `traceId` viaja entre servicios en la cabecera `traceparent` (W3C Trace Context), por HTTP y dentro de los mensajes de RabbitMQ |
| 4. ¿Cómo consume Grafana los datos? | Con fuentes de datos aprovisionadas desde archivos: consulta métricas a Prometheus (PromQL), logs a Loki (LogQL) y trazas a Jaeger. Evalúa las reglas de alerta y notifica por email a Mailhog |
