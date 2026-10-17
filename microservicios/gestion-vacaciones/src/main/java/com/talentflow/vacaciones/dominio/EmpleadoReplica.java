package com.talentflow.vacaciones.dominio;

/**
 * Copia local mínima de un empleado, alimentada por eventos (decisión (b) del reto:
 * réplica por eventos). Permite validar la existencia del empleado sin llamar por REST a
 * empleados-service, y trae el email que exige el evento vacaciones.programadas.
 */
public record EmpleadoReplica(String empleadoId, String email, boolean retirado) {
}
