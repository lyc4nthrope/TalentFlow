package com.talentflow.vacaciones.aplicacion;

import com.talentflow.vacaciones.dominio.ConflictoDeEstadoException;
import com.talentflow.vacaciones.dominio.EmpleadoReplica;
import com.talentflow.vacaciones.dominio.ReglasVacaciones;
import com.talentflow.vacaciones.dominio.SolicitudInvalidaException;
import com.talentflow.vacaciones.dominio.Vacaciones;
import com.talentflow.vacaciones.dominio.VacacionesNoEncontradasException;
import java.time.Clock;
import java.time.Instant;
import java.time.LocalDate;
import java.time.temporal.ChronoUnit;
import java.util.List;

/** Casos de uso de RR. HH. sobre los períodos de vacaciones. */
public class ServicioVacaciones {

    private final RepositorioVacaciones vacaciones;
    private final RepositorioEmpleados empleados;
    private final PublicadorEventos publicador;
    private final Transacciones transacciones;
    private final Clock reloj;

    public ServicioVacaciones(RepositorioVacaciones vacaciones, RepositorioEmpleados empleados,
            PublicadorEventos publicador, Transacciones transacciones, Clock reloj) {
        this.vacaciones = vacaciones;
        this.empleados = empleados;
        this.publicador = publicador;
        this.transacciones = transacciones;
        this.reloj = reloj;
    }

    private record Programacion(Vacaciones vacaciones, String email) {
    }

    public Vacaciones programar(SolicitudVacaciones solicitud) {
        // Validaciones 1 y 2 (no requieren BD).
        ReglasVacaciones.validarFechas(solicitud.fechaInicio(), solicitud.fechaFin(), LocalDate.now(reloj));

        Programacion programacion = transacciones.ejecutar(() -> {
            // Validación 4: el empleado debe existir en la réplica (y no estar retirado).
            EmpleadoReplica empleado = empleados.buscarParaProgramar(solicitud.empleadoId())
                    .orElseThrow(() -> new SolicitudInvalidaException("empleadoId",
                            "El empleado " + solicitud.empleadoId() + " no corresponde a un empleado registrado"));
            if (empleado.retirado()) {
                throw new SolicitudInvalidaException("empleadoId",
                        "El empleado " + solicitud.empleadoId() + " está retirado y no puede programar vacaciones");
            }

            // Validación 3: sin solapamiento con un período PROGRAMADA o EN_CURSO.
            vacaciones.buscarVigenteSolapado(solicitud.empleadoId(), solicitud.fechaInicio(), solicitud.fechaFin())
                    .ifPresent(conflicto -> {
                        throw new SolicitudInvalidaException("fechaInicio",
                                "El período se cruza con otro período " + conflicto.estado() + " del empleado ("
                                        + conflicto.id() + ": " + conflicto.fechaInicio() + " a "
                                        + conflicto.fechaFin() + ")",
                                conflicto);
                    });

            Instant ahora = Instant.now(reloj).truncatedTo(ChronoUnit.SECONDS);
            Vacaciones creada = vacaciones.crear(
                    solicitud.empleadoId(), solicitud.fechaInicio(), solicitud.fechaFin(), ahora);
            return new Programacion(creada, empleado.email());
        });

        // Después del commit: si la publicación falla, el período queda registrado igual
        // (el publicador deja el error en el log). Mismo contrato que empleados-service.
        Vacaciones creada = programacion.vacaciones();
        publicador.publicar(VacacionesProgramadas.TIPO, new VacacionesProgramadas(
                creada.id(),
                creada.empleadoId(),
                programacion.email(),
                creada.fechaInicio(),
                creada.fechaFin(),
                ReglasVacaciones.diasHabiles(creada.fechaInicio(), creada.fechaFin())));
        return creada;
    }

    public Vacaciones consultar(String id) {
        return vacaciones.buscarPorId(id).orElseThrow(() -> new VacacionesNoEncontradasException(id));
    }

    public List<Vacaciones> listar(String empleadoId) {
        return empleadoId == null ? vacaciones.listar() : vacaciones.listarPorEmpleado(empleadoId);
    }

    /** Cancela un período que aún no ha iniciado. No se borra: queda CANCELADA. */
    public Vacaciones cancelar(String id) {
        return transacciones.ejecutar(() -> {
            Vacaciones actual = consultar(id);
            if (!actual.esCancelable(LocalDate.now(reloj))) {
                throw new ConflictoDeEstadoException("El período " + id + " no se puede cancelar: está "
                        + actual.estado() + (actual.estaVigente() ? " y ya inició" : ""));
            }
            if (!vacaciones.cancelar(id)) {
                throw new ConflictoDeEstadoException("El período " + id + " cambió de estado y no se puede cancelar");
            }
            return actual.cancelada();
        });
    }
}
