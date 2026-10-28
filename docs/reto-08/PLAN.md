# Reto 8 — Plan de implementación

Fuente de requisitos: `reto08.pdf` (secciones 1 a 7, entregables, consideraciones y criterios) y la teoría de clase `observabilidad.pdf`.
Punto de partida: `lyc4nthrope` con los Retos 1 a 7 completos (igual a `dev` y `main`).
Cada fase termina **verificada** (en la UI de Prometheus, Grafana, Jaeger o Loki, no solo en la consola), con su propio commit.

## Diagnóstico de partida (2026-10-28)

| Requisito | Estado actual | Brecha |
|---|---|---|
| `/health` en cada servicio, 200/503 (4.3) | ✅ Existe desde los Retos 3 a 5, con `components: {app, db, broker}` | Falta el formato del PDF: `checks: {database, messageBroker}` |
| `/metrics` en formato Prometheus: CPU, memoria, peticiones por código y latencia (4.1) | ❌ Ningún servicio (no hay `prom-client`, `client_golang`, `micrometer` ni equivalentes) | Los 7 servicios |
| OpenTelemetry con `service.name`, exportador y propagación W3C (4.2) | ❌ Nada | Los 7 servicios |
| Traza a través del broker y span raíz del scheduler (4.2, "el caso difícil") | ❌ | Publicadores y consumidores de RabbitMQ en 5 lenguajes |
| Logs JSON con `traceId` (4.4) | ❌ Texto libre con emojis (`📤 Evento publicado: …`) | Los 7 servicios |
| Prometheus, Grafana, Loki, Promtail y Zipkin/Jaeger en Compose, misma red (2) | ❌ | Todo |
| Dashboard aprovisionado: 2 filas, 4 paneles (5) | ❌ | Todo |
| Al menos 2 alertas y un canal externo (6) | ❌ | Todo |
| Pruebas de caos con capturas y la pregunta de análisis (7) | ❌ | Todo |
| Puerto 9090 para Prometheus | ⚠️ Lo usa Jenkins desde el Reto 7 | Elegir otro puerto en el host |

## Decisiones de diseño

| # | Decisión | Elección | Por qué |
|---|---|---|---|
| D1 | Servidor de trazas | **Jaeger** (all-in-one) recibiendo **OTLP** | OTLP es el protocolo propio de OpenTelemetry: los 7 servicios exportan igual, sin un exportador Zipkin distinto por lenguaje, y cambiar de servidor después no toca el código. Jaeger es liviano y su UI compara trazas y muestra el camino crítico. El PDF permite elegir |
| D2 | Puertos en el host | Prometheus en **9091** (dentro de la red sigue en 9090); Grafana 3000; Jaeger UI 16686; Loki 3100 | Jenkins conserva el 9090 que exige el Reto 7. Dentro de Docker nada cambia |
| D3 | Cuándo se levanta | **Con `docker compose up`**, sin perfil | La observabilidad es parte del sistema en producción (a diferencia de la CI del Reto 7, que es una herramienta aparte). El PDF prueba todo con `docker compose up --build` |
| D4 | Formato de `/health` | Se **agrega** `checks: {database, messageBroker}` con el formato del PDF y se conservan los campos actuales | El agregador del Gateway (Reto 3) y los healthchecks de Docker siguen funcionando |
| D5 | Contrato de métricas HTTP | En los 7 servicios: `http_requests_total` (etiquetas `method`, `route`, `status`) y el histograma `http_request_duration_seconds`, más las métricas de proceso de cada librería | Una sola consulta PromQL por panel del dashboard, en lugar de una por lenguaje. `route` es la ruta con parámetros (`/empleados/:id`), no la URL real, para no disparar la cardinalidad |
| D6 | Todo como código | Fuentes de datos (Prometheus, Loki, Jaeger), dashboard, reglas de alerta y canal de notificación, aprovisionados desde archivos en `observability/` | Como en el Reto 7: al levantar, todo está listo, sin clics. El PDF lo exige para las fuentes de datos y el dashboard |
| D7 | Dónde va la instrumentación | En arranque, adaptadores HTTP y de mensajería, nunca en el dominio ni en los casos de uso | Mantener el 100 % de cobertura de la lógica de negocio del Reto 6 sin probar código de infraestructura |
| D8 | Canal de alertas | **Email por Mailhog** (SMTP de Grafana → `mailhog:1025`) | El PDF lo acepta, ya está en el sistema, no necesita cuentas ni secretos y funciona igual en la máquina del profesor y en el Mac de Valeria. Las alertas se ven en `http://localhost:8025` |
| D9 | Reglas de alerta | Servicio caído (`up == 0` durante 1 min), alta tasa de 5xx (> 10 % durante 2 min) y alta latencia (> 2 s promedio durante 2 min) | Las tres que se miden con nuestras métricas. La de reinicios de contenedor necesitaría cAdvisor, otra pieza más |
| D10 | Propagación por el broker | El publicador inyecta `traceparent` en las cabeceras del mensaje AMQP y el consumidor lo extrae antes de procesar; el scheduler crea el span raíz a mano | La propagación por HTTP la hace el SDK; por el broker casi nunca. Sin esto, la mitad asíncrona del sistema queda fuera de las trazas (PDF 4.2) |
| D11 | Versiones | Imágenes y librerías en versiones exactas (no `latest`) | Que el entorno sea reproducible; el PDF da `latest` solo como referencia |
| D12 | Muestreo de trazas | 100 % | La teoría de clase lo indica para desarrollo y QA; en producción se usaría muestreo por cabecera |

## Fases

| Fase | Alcance | Verificación | Estado |
|---|---|---|---|
| **F0** Plan y diagrama | Este plan; [`arquitectura.md`](arquitectura.md) con el diagrama del PDF (paso 1) | Diagrama con los 4 puntos que pide el PDF | ✅ |
| **F1** Infraestructura | D1-D3, D6, D11: Prometheus, Grafana, Loki, Promtail y Jaeger en Compose, en la misma red; fuentes de datos aprovisionadas; volúmenes persistentes | Las 5 UIs responden; Grafana trae las 3 fuentes de datos sin configurar nada | ✅ |
| **F2** Métricas y salud en Node | D4, D5, D7: Gateway y empleados | Targets UP en Prometheus; `/health` con `checks`; Reto 6 en verde | ⬜ |
| **F3** Métricas y salud en Go | auth y notificaciones | Igual que F2 | ⬜ |
| **F4** Métricas y salud en Python y Java | perfiles y vacaciones (Actuator + Micrometer) | Igual que F2 | ⬜ |
| **F5** Métricas y salud en PHP | departamentos: los contadores necesitan un almacenamiento compartido entre peticiones | Igual que F2; los 7 targets UP | ⬜ |
| **F6** Trazas HTTP | OpenTelemetry en los 7, `service.name`, exportador OTLP, W3C | En Jaeger: Gateway → empleados → departamentos en una sola traza | ⬜ |
| **F7** Trazas por el broker | D10: publicadores y consumidores; span raíz del scheduler | Una sola cascada para el alta (… → auth → notificaciones y perfiles) y para el inicio de vacaciones (scheduler → auth → notificaciones) | ⬜ |
| **F8** Logs estructurados | JSON con `timestamp`, `level`, `service`, `traceId`, `message` en los 7; Loki como fuente de datos | Una consulta LogQL por `traceId` devuelve los logs de varios servicios | ⬜ |
| **F9** Dashboard | D5, D6: fila "Resumen del sistema" (estado) y fila "Comportamiento del tráfico" (tasa, latencia, errores) | El dashboard aparece solo al levantar y muestra datos reales | ⬜ |
| **F10** Alertas | D8, D9: reglas, canal de email y política de notificación como código | Una alerta forzada llega a Mailhog | ⬜ |
| **F11** Pruebas de caos | PDF 7: apagar un servicio, inducir latencia, capturas, la pregunta "¿qué servicio tardó más?" | Evidencia en `pruebas.md` con capturas | ⬜ |
| **F12** Documentación | README raíz (sección del Reto 8, índice, mapa), manual y sustentación | Cada entregable del PDF presente | ⬜ |
| **F13** Cierre | Pruebas de los Retos 6 y 7 en verde, PR `lyc4nthrope` → `dev` → `main` | Ramas sincronizadas | ⬜ |

## Fuera de alcance (anotado para después)

- La alerta de "contenedor reiniciado": necesitaría cAdvisor o el exportador de Docker.

## Aprendizajes por fase

- **F0**: el PDF pide el diagrama antes de implementar. Se detectó que el puerto 9090 de Prometheus choca con el de Jenkins (Reto 7) y que ningún servicio expone métricas ni trazas todavía.
- **F1**: versiones fijadas: `prom/prometheus:v3.15.1`, `grafana/grafana:12.4.14` (OSS), `grafana/loki:3.6.19`, `grafana/promtail:3.6.11` y `jaegertracing/jaeger:2.20.0`. Jaeger v2 es un colector de OpenTelemetry por dentro: recibe OTLP en 4317/4318 sin adaptadores, y esos puertos quedan solo en la red (`expose`). Se fijó en **2.20.0** y no en la última (2.22.0) porque desde la 2.21 Jaeger quitó la API HTTP v1 (`/api/services`, `/api/traces`) y la fuente de datos Jaeger de Grafana 12 todavía la usa: con 2.22 la prueba de salud de la fuente daba 404. Promtail 3.6.11 es su última versión (Grafana lo reemplaza por Alloy); el PDF lo pide y es compatible con Loki 3.6. Promtail lee los logs por la API de Docker (`docker_sd_configs`), así que basta montar el socket en solo lectura; no necesita `/var/lib/docker/containers` y funciona igual en Linux y en Docker Desktop de Mac/Windows (allí requiere la opción del socket por defecto, la misma que ya usa Jenkins). Solo recoge los proyectos `talentflow*` y agrega la etiqueta `project`, que separa el sistema de desarrollo de las E2E de la CI (`talentflow-ci-<build>`). La primera vez lee el log completo de cada contenedor: se descarta lo de más de 24 h para que Loki no lo rechace por viejo. En la E2E de la CI la observabilidad no se levanta (perfil sin activar en `docker-compose.ci-e2e.yml`): chocarían los `container_name` y los puertos. Los puertos del host son configurables (`PROMETHEUS_PORT`, `GRAFANA_PORT`, `LOKI_PORT`, `JAEGER_UI_PORT`) porque en esta máquina otro proyecto ya ocupaba el 3000 y el 9091.
