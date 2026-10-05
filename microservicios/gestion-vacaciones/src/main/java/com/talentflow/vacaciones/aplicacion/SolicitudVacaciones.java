package com.talentflow.vacaciones.aplicacion;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;
import java.time.LocalDate;

/** Cuerpo de POST /vacaciones. Formato y obligatoriedad; las reglas de negocio van en el dominio. */
public record SolicitudVacaciones(
        @NotBlank @Size(max = 20) String empleadoId,
        @NotNull LocalDate fechaInicio,
        @NotNull LocalDate fechaFin) {
}
