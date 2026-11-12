package com.talentflow.vacaciones.aplicacion;

import com.fasterxml.jackson.databind.annotation.JsonDeserialize;
import io.swagger.v3.oas.annotations.media.Schema;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;

/**
 * Cuerpo de PUT /vacaciones/{id}: nuevas fechas con el mismo formato que POST (ver
 * {@link FechaHoraEntrada}). empleadoId es opcional; si llega, debe ser el del período.
 */
public record ReprogramacionVacaciones(
        @Size(max = 20)
        @Schema(example = "E001", description = "Opcional. Si se envía debe ser el empleado del período: no se puede cambiar")
        String empleadoId,
        @NotNull
        @JsonDeserialize(using = FechaHoraEntrada.Inicio.class)
        @Schema(type = "string", example = "2026-11-02T08:00:00",
                description = "Fecha y hora local (America/Bogota). Solo YYYY-MM-DD = 00:00:00 de ese día (si es hoy, desde ahora)")
        FechaSolicitada fechaInicio,
        @NotNull
        @JsonDeserialize(using = FechaHoraEntrada.Fin.class)
        @Schema(type = "string", example = "2026-11-06T18:00:00",
                description = "Fecha y hora local (America/Bogota), posterior a fechaInicio. Solo YYYY-MM-DD = 23:59:59 de ese día")
        FechaSolicitada fechaFin) {
}
