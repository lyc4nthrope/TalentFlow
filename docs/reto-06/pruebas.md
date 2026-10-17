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
