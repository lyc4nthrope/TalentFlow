package com.talentflow.vacaciones.aplicacion;

import com.talentflow.vacaciones.dominio.Vacaciones;
import java.time.Instant;
import java.time.LocalDate;
import java.util.List;
import java.util.Optional;

/** Puerto de persistencia de los períodos de vacaciones. */
public interface RepositorioVacaciones {

    /** Crea un período PROGRAMADA y le asigna su id (V-AAAA-NNNN). */
    Vacaciones crear(String empleadoId, LocalDate fechaInicio, LocalDate fechaFin, Instant fechaCreacion);

    Optional<Vacaciones> buscarPorId(String id);

    List<Vacaciones> listar();

    List<Vacaciones> listarPorEmpleado(String empleadoId);

    /** Primer período vigente (PROGRAMADA o EN_CURSO) del empleado que se cruza con el rango. */
    Optional<Vacaciones> buscarVigenteSolapado(String empleadoId, LocalDate fechaInicio, LocalDate fechaFin);

    /** Pasa a CANCELADA solo si sigue PROGRAMADA. Devuelve false si otra operación lo cambió antes. */
    boolean cancelar(String id);
}
