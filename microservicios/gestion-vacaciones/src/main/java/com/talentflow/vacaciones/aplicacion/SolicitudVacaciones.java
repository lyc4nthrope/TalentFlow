package com.talentflow.vacaciones.aplicacion;

import com.fasterxml.jackson.databind.annotation.JsonDeserialize;
import io.swagger.v3.oas.annotations.media.Schema;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;

/**
 * Cuerpo de POST /vacaciones. Formato y obligatoriedad; las reglas de negocio van en el dominio.
 * Las fechas llevan hora (Reto 5, D6); ver {@link FechaHoraEntrada}.
 */
public record SolicitudVacaciones(
        @NotBlank @Size(max = 20) String empleadoId,
        @NotNull
        @JsonDeserialize(using = FechaHoraEntrada.Inicio.class)
        @Schema(type = "string", example = "2026-10-20T08:00:00",
                description = "Fecha y hora local (America/Bogota). Solo YYYY-MM-DD = 00:00:00 de ese día (si es hoy, desde ahora)")
        FechaSolicitada fechaInicio,
        @NotNull
        @JsonDeserialize(using = FechaHoraEntrada.Fin.class)
        @Schema(type = "string", example = "2026-10-24T18:00:00",
                description = "Fecha y hora local (America/Bogota), posterior a fechaInicio. Solo YYYY-MM-DD = 23:59:59 de ese día")
        FechaSolicitada fechaFin) {
}
