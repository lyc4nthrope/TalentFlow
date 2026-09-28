package com.talentflow.vacaciones.aplicacion;

import com.talentflow.vacaciones.dominio.EmpleadoReplica;
import java.util.Optional;

/** Puerto de la réplica local de empleados (alimentada por eventos). */
public interface RepositorioEmpleados {

    /**
     * Busca al empleado y bloquea su fila hasta el fin de la transacción: dos solicitudes
     * simultáneas para el mismo empleado se atienden en serie, así no pueden pasar ambas
     * la validación de solapamiento.
     */
    Optional<EmpleadoReplica> buscarParaProgramar(String empleadoId);

    /** empleado.creado: registra al empleado si no existía. */
    void registrar(String empleadoId, String email);

    /** empleado.actualizado: mantiene el email al día (lo crea si su evento de alta se perdió). */
    void actualizarEmail(String empleadoId, String email);

    /** empleado.retirado: ya no puede programar vacaciones. */
    void marcarRetirado(String empleadoId, String email);
}
