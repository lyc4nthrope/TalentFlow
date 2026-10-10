package com.talentflow.notificaciones.dominio;

import io.swagger.v3.oas.annotations.media.Schema;
import java.time.Instant;

@Schema(description = "Notificación registrada (el envío del correo es simulado por log)")
public record Notificacion(
        @Schema(example = "7c9e6679-7425-40de-944b-e07fc1f90ae7") String id,
        @Schema(example = "BIENVENIDA") TipoNotificacion tipo,
        @Schema(example = "juan.perez@empresa.com") String destinatario,
        @Schema(example = "Bienvenido Juan Pérez. Su registro como empleado fue exitoso.") String mensaje,
        @Schema(example = "2026-10-10T12:00:00Z") Instant fechaEnvio,
        @Schema(example = "E001") String empleadoId) {
}
