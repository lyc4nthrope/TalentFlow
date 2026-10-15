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

    /** Primer período vigente (PROGRAMADA o EN_CURSO) del empleado que se cruza con el rango [inicio, fin). */
    Optional<Vacaciones> buscarVigenteSolapado(String empleadoId, LocalDateTime fechaInicio, LocalDateTime fechaFin);

    /** Pasa a CANCELADA solo si sigue PROGRAMADA. Devuelve false si otra operación lo cambió antes. */
    boolean cancelar(String id);

    /** Períodos PROGRAMADA cuya fechaInicio ya llegó (fechaInicio <= ahora), del más antiguo al más reciente. */
    List<Vacaciones> listarPorIniciar(LocalDateTime ahora);

    /** Períodos EN_CURSO cuya fechaFin ya llegó (fechaFin <= ahora), del más antiguo al más reciente. */
    List<Vacaciones> listarPorFinalizar(LocalDateTime ahora);

    /**
     * Cambia el estado solo si el período sigue en el estado esperado. Devuelve false si otra
     * operación (una cancelación, otra pasada del scheduler) lo cambió antes: así nunca se
     * publica dos veces la misma transición.
     */
    boolean cambiarEstado(String id, EstadoVacaciones esperado, EstadoVacaciones nuevo);
}
