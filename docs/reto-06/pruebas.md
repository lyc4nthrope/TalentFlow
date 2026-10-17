# Reto 6 — Evidencia de las pruebas

## 1. Las pruebas fallan cuando deben (Punto 1, paso 6)

Se rompió a propósito una regla de negocio, se ejecutaron las pruebas unitarias y se revirtió el cambio. Ninguno de estos cambios quedó versionado.

### 1.1 Vacaciones (Java): invertir la comparación de fechas

Cambio en `ReglasVacaciones.validarFechas` (`dominio/ReglasVacaciones.java:21`):

```diff
-        if (!fechaFin.isAfter(fechaInicio)) {
+        if (!fechaFin.isBefore(fechaInicio)) {
```

Resultado de `mvn test`: código de salida **1**, `Tests run: 129, Failures: 13, Errors: 18`. En `ReglasVacacionesTest` fallan exactamente los casos de la regla, cada uno con su nombre de la tabla (tomado de `reports/junit/TEST-com.talentflow.vacaciones.dominio.ReglasVacacionesTest.xml`):

```text
FALLA: aceptaFechasCoherentesDesdeAhora [1] empieza ahora mismo → válido
FALLA: aceptaFechasCoherentesDesdeAhora [2] período de minutos el mismo día → válido
FALLA: aceptaFechasCoherentesDesdeAhora [3] fin un segundo después del inicio → válido
FALLA: rechazaFechasIncoherentesOPasadas [1] fin antes del inicio → 400 en fechaFin
FALLA: rechazaFechasIncoherentesOPasadas [3] fin un segundo antes del inicio → 400 en fechaFin
FALLA: rechazaFechasIncoherentesOPasadas [4] inicio ayer → 400 en fechaInicio
FALLA: rechazaFechasIncoherentesOPasadas [5] inicio hoy pero una hora antes de ahora → 400 en fechaInicio
FALLA: rechazaFechasIncoherentesOPasadas [6] inicio un segundo antes de ahora → 400 en fechaInicio
FALLA: rechazaFechasIncoherentesOPasadas [7] fin incoherente y en el pasado: gana fechaFin → 400 en fechaFin
```

Y el mensaje dice qué se esperaba y qué pasó:

```text
Expecting code not to raise a throwable but caught
  "SolicitudInvalidaException: La fechaFin (2026-10-20T10:30) debe ser posterior a la fechaInicio (2026-10-15T10:30)"
```

Los otros fallos de la corrida (en `ServicioVacacionesTest` y en el ciclo del scheduler) son consecuencia de la misma regla: con ella invertida, ninguna solicitud válida se puede programar.

### 1.2 auth-service (Go): invertir el vencimiento del token

Cambio en `tokenVigente` (`internal/aplicacion/servicio.go:291`):

```diff
-	return tipoValido && s.ahora().Before(u.TokenExpiraEn)
+	return tipoValido && s.ahora().After(u.TokenExpiraEn)
```

Fallaron 43 pruebas, porque casi todos los flujos activan una cuenta en su preparación. Las específicas del vencimiento dicen exactamente qué se rompió:

```text
--- FAIL: TestTokenVigenteUnSegundoAntesDeVencer
    servicio_test.go:242: err = token inválido o expirado
--- FAIL: TestTokenDeRecuperacionVenceEnUnaHora
    servicio_test.go:430: token inválido o expirado
```

**Lo que este ejercicio encontró.** `TestTokenExpiradoSeRechaza` **siguió pasando** con la regla invertida. Avanzaba el reloj exactamente 24 h, el instante del vencimiento, y en ese instante la regla correcta y la invertida rechazan el token por igual. Ninguna prueba miraba un token vencido hace rato, que es justo el caso que la regla invertida acepta. Se agregó `TestTokenVencidoHaceRatoSeRechaza` (48 h) y, repitiendo el mismo cambio, ahora la regla invertida queda al descubierto:

```text
    servicio_test.go:245: token vencido hace 24 h: err = <nil>
--- FAIL: TestTokenVencidoHaceRatoSeRechaza
```

Es el ejemplo concreto de "la trampa de la cobertura" del PDF: la línea ya estaba cubierta al 100 % y aun así una prueba de borde no distinguía la regla correcta de su inversa.

### 1.3 auth-service (Go): una regla con defensa doble

También se probó quitar la condición que impide reactivar una cuenta retirada al llegar `vacaciones.finalizadas` (`internal/aplicacion/ciclo.go:162`). **Las pruebas siguieron pasando, y es lo correcto.** La regla está protegida dos veces: por esa condición y por el repositorio, que solo reactiva una cuenta `SUSPENDIDA_TEMPORAL` por esas mismas vacaciones (D18 del Reto 5). Al quitar una capa, el comportamiento observable sigue siendo el correcto (la cuenta queda retirada y no se publica `cuenta.activada`), y eso es lo que verifican las pruebas: el comportamiento, no la línea.

## 2. Reproducibilidad de la suite BDD (Punto 6, pasos 1, 4 y 5)

### 2.1 Comandos únicos

`Makefile` en la raíz (`make` sin objetivo lista los objetivos):

| Comando | Qué hace |
|---|---|
| `make pruebas` | Unitarias e integración de cada microservicio, en orden: empleados y api-gateway (`npm test`), auth, notificaciones y perfiles (`make test`), vacaciones (`mvn verify`, con Testcontainers) y el soporte de la suite (`npm run test:unidad`). Se detiene en la primera que falla |
| `make e2e` | `docker compose -f docker-compose.yml -f docker-compose.e2e.yml up -d --build` (el override pone el scheduler de vacaciones cada 5 s), espera con polling a que `GET /health` diga `"sistema":"OK"` y ejecuta la suite en el contenedor `bdd-tests` |
| `make e2e-local` | Lo mismo, pero la suite corre con el Node de la máquina (`npm test` en `e2e-tests/`) |
| `make e2e-restaurar` | `docker compose up -d vacaciones-service`: vuelve al cron por defecto (cada minuto) |

Contenedor `bdd-tests` (D13): definido en `docker-compose.e2e.yml`, en el perfil `e2e` (un `docker compose up` no lo levanta), imagen `node:24.14.0-alpine` con `npm ci`; `BASE_URL=http://api-gateway:8080`, `MAILHOG_URL=http://mailhog:8025`, `E2E_SCHEDULER_SEGUNDOS=5`; espera a api-gateway y vacaciones-service `healthy`; `./e2e-tests/reports` montado en `/app/reports`. Corre con el uid/gid de quien ejecuta `make` (`E2E_UID`/`E2E_GID`, por defecto 1000, el usuario `node`): los reportes quedan del usuario del host, no de root (comprobado también con uid 1234). Como `reports/` es ahora un punto de montaje, `pretest` vacía su contenido en lugar de borrar la carpeta.

Verificado: `make pruebas` en verde en 29 s con todos los reportes en su lugar; con `make pruebas PYTHON=false` se detiene en perfiles (`make: *** [Makefile:61: pruebas-perfiles] Error 2`) y no llega a vacaciones. `make e2e-restaurar` deja `VACACIONES_CRON=0 * * * * *` y `make e2e` vuelve a `*/5 * * * * *`.

### 2.2 Tres ejecuciones consecutivas

`make e2e` tres veces seguidas, sin tocar nada entre corridas:

| Corrida | Resultado | Suite (`time` del `junit.xml`) | `make e2e` completo | `GET /notificaciones` antes | Correos en Mailhog antes |
|---|---|---|---|---|---|
| 1 | 61/61, 313 pasos, código 0 | 70,4 s | 79,3 s | 526 103 B, 8,5 ms | 1814 |
| 2 | 61/61, 313 pasos, código 0 | 70,5 s | 79,7 s | 553 596 B, 8,8 ms | 1909 |
| 3 | 61/61, 313 pasos, código 0 | 76,0 s | 84,7 s | 581 088 B, 10,5 ms | 2004 |

Después: 608 578 B (11,5 ms) y 2099 correos. Se agregaron otras tres corridas completas por el contenedor (66,7 s, 73,7 s y 73,6 s) y una con `make e2e-local` (72,5 s): siete corridas seguidas en verde, sin ningún escenario intermitente.

**¿Crece la duración?** No de forma que importe. Cada corrida agrega ≈ 27,5 KB al historial y 95 correos, y la consulta del historial completo pasa de 8,5 a 11,5 ms; solo la usa el escenario de la recuperación (control positivo y ventana de ausencia, unas pocas consultas), que tardó 5,48 s, 5,55 s y 5,52 s. Las diferencias entre corridas salen de otro lado (duración por escenario en `cucumber.json`):

- El ciclo de vacaciones depende de la fase del scheduler de 5 s: 15,9 s, 14,8 s y 18,1 s (el retiro durante vacaciones 20,0 s, 18,9 s y 19,9 s). Explica casi todo el +5,5 s de la corrida 3.
- Los escenarios que esperan una activación tardan ≈ 0,25 s o ≈ 1,27 s según si la primera consulta llega antes que el evento (paga un intervalo de polling de 1 s, ya medido en F7/F8), y eso cambia al azar entre corridas.

No se cambió la suite. Proyección: a este ritmo el historial pasaría 3 MB después de unas 100 corridas más, con consultas de decenas de ms; si llega a pesar, el arreglo es un filtro por destinatario en notificaciones-service (anotado en "Fuera de alcance" del plan), porque la notificación de recuperación no lleva `empleadoId` y no sale en `/notificaciones/{id}`. Las demás consultas de la suite ya usan `/notificaciones/{empleadoId}` y la búsqueda de Mailhog por destinatario (1–2 ms con 1800 correos).

### 2.3 Fallo simulado

Cambio (revertido con `git checkout` después de la corrida) en `features/seguridad/autorizacion.feature:45`, el último paso de "Un empleado con rol USER solo puede editar su propio perfil":

```diff
-    Entonces la respuesta debe tener código 403
+    Entonces la respuesta debe tener código 200
```

Consola de `make e2e`:

```text
    ✘ Entonces la respuesta debe tener código 200    # step_definitions/comunes.steps.js:7
          Error: Se esperaba código 200 y se obtuvo 403 en PUT /perfiles/E-mvcwiiel-6e077d. Cuerpo: {"error":"FORBIDDEN","message":"No tiene permisos para realizar esta acción.","path":"/perfiles/E-mvcwiiel-6e077d"}
61 scenarios (60 passed, 1 failed)
313 steps (312 passed, 1 failed)
make: *** [Makefile:85: e2e] Error 1
```

Código de salida: el contenedor termina en 1, `docker compose run` lo devuelve y `make` termina en **2** (≠ 0, lo que hará fallar el pipeline).

`reports/junit.xml`:

```xml
<testsuite name="Cucumber" time="77.696" tests="61" skipped="0" failures="1" errors="0" …>
  <failure type="Error" message="Se esperaba código 200 y se obtuvo 403 en PUT /perfiles/E-mvcwiiel-6e077d. Cuerpo: {&quot;error&quot;:&quot;FORBIDDEN&quot;,…}">
```

`reports/cucumber.json`: el único paso no `passed` es `features/seguridad/autorizacion.feature | Un empleado con rol USER solo puede editar su propio perfil | Entonces la respuesta debe tener código 200 | failed`. `reports/cucumber.html` tiene exactamente un `"status":"FAILED"`: el escenario aparece en rojo con el mensaje. Con el cambio revertido, `make e2e` vuelve a 61/61 y código 0.
