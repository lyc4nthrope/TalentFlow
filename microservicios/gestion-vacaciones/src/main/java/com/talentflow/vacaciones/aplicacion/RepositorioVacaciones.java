package com.talentflow.vacaciones.aplicacion;

import com.talentflow.vacaciones.dominio.EstadoVacaciones;
import com.talentflow.vacaciones.dominio.Vacaciones;

import java.time.Instant;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;

/** Puerto de persistencia de los períodos de vacaciones. */
public interface RepositorioVacaciones {

    /** Crea un período PROGRAMADA y le asigna su id (V-AAAA-NNNN). */
    Vacaciones crear(String empleadoId, LocalDateTime fechaInicio, LocalDateTime fechaFin, Instant fechaCreacion);

    Optional<Vacaciones> buscarPorId(String id);

    List<Vacaciones> listar();

    List<Vacaciones> listarPorEmpleado(String empleadoId);

    /** Primer período vigente (PROGRAMADA o EN_CURSO) del empleado que se cruza con el rango. */
    Optional<Vacaciones> buscarVigenteSolapado(String empleadoId, LocalDateTime fechaInicio, LocalDateTime fechaFin);

    /** Pasa a CANCELADA solo si sigue PROGRAMADA. Devuelve false si otra operación lo cambió antes. */
    boolean cancelar(String id);

    // Encuentra vacaciones PROGRAMADAS con fechaInicio <= ahora
    List<Vacaciones> findByEstadoAndFechaInicioLessThanEqual(EstadoVacaciones estado, LocalDateTime fecha);

    // Encuentra vacaciones EN_CURSO con fechaFin <= ahora
    List<Vacaciones> findByEstadoAndFechaFinLessThanEqual(EstadoVacaciones estado, LocalDateTime fecha);

    // Actualiza el estado de un período de vacaciones (PROGRAMADA -> EN_CURSO, EN_CURSO -> FINALIZADA)
    boolean actualizarEstado(String id, EstadoVacaciones nuevoEstado);

    // Actualiza el registro de vacaciones completo en la base de datos
    boolean actualizar(Vacaciones vacaciones);
}
