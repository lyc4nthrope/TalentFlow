package com.talentflow.vacaciones.aplicacion;

import com.talentflow.vacaciones.dominio.ConflictoDeEstadoException;
import com.talentflow.vacaciones.dominio.EmpleadoReplica;
import com.talentflow.vacaciones.dominio.ReglasVacaciones;
import com.talentflow.vacaciones.dominio.SolicitudInvalidaException;
import com.talentflow.vacaciones.dominio.Vacaciones;
import com.talentflow.vacaciones.dominio.VacacionesNoEncontradasException;
import java.time.Clock;
import java.time.Instant;
import java.time.LocalDateTime;
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
        // Validaciones 1 y 2 (no requieren BD). Fechas con hora, comparadas con ahora (D6).
        LocalDateTime ahora = LocalDateTime.now(reloj).truncatedTo(ChronoUnit.SECONDS);
        LocalDateTime fechaInicio = FechaHoraEntrada.inicioEfectivo(solicitud.fechaInicio(), ahora);
        LocalDateTime fechaFin = solicitud.fechaFin().valor();
        ReglasVacaciones.validarFechas(fechaInicio, fechaFin, ahora);

        Programacion programacion = transacciones.ejecutar(() -> {
            EmpleadoReplica empleado = empleadoHabilitado(solicitud.empleadoId());
            // Validación 3: sin solapamiento con un período PROGRAMADA o EN_CURSO.
            vacaciones.buscarVigenteSolapado(solicitud.empleadoId(), fechaInicio, fechaFin)
                    .ifPresent(ServicioVacaciones::rechazarSolapamiento);

            Vacaciones creada = vacaciones.crear(solicitud.empleadoId(), fechaInicio, fechaFin,
                    Instant.now(reloj).truncatedTo(ChronoUnit.SECONDS));
            return new Programacion(creada, empleado.email());
        });

        // Después del commit: si la publicación falla, el período queda registrado igual
        // (el publicador deja el error en el log). Mismo contrato que empleados-service.
        Vacaciones creada = programacion.vacaciones();
        publicador.publicar(VacacionesProgramadas.TIPO,
                VacacionesProgramadas.de(creada, programacion.email(), reloj.getZone()));
        return creada;
    }

    /** Validación 4: el empleado debe existir en la réplica (y no estar retirado). Bloquea su fila. */
    private EmpleadoReplica empleadoHabilitado(String empleadoId) {
        EmpleadoReplica empleado = empleados.buscarParaProgramar(empleadoId)
                .orElseThrow(() -> new SolicitudInvalidaException("empleadoId",
                        "El empleado " + empleadoId + " no corresponde a un empleado registrado"));
        if (empleado.retirado()) {
            throw new SolicitudInvalidaException("empleadoId",
                    "El empleado " + empleadoId + " está retirado y no puede programar vacaciones");
        }
        return empleado;
    }

    private static void rechazarSolapamiento(Vacaciones conflicto) {
        throw new SolicitudInvalidaException("fechaInicio",
                "El período se cruza con otro período " + conflicto.estado() + " del empleado ("
                        + conflicto.id() + ": " + conflicto.fechaInicio() + " a " + conflicto.fechaFin() + ")",
                conflicto);
    }

    /**
     * Cambia las fechas de un período PROGRAMADA que aún no inicia, con las mismas reglas que
     * programar (sin chocar consigo mismo). Publica de nuevo vacaciones.programadas: el
     * catálogo no tiene un evento de "actualizadas" y así el empleado recibe las fechas nuevas.
     */
    public Vacaciones reprogramar(String id, ReprogramacionVacaciones solicitud) {
        LocalDateTime ahora = LocalDateTime.now(reloj).truncatedTo(ChronoUnit.SECONDS);
        LocalDateTime fechaInicio = FechaHoraEntrada.inicioEfectivo(solicitud.fechaInicio(), ahora);
        LocalDateTime fechaFin = solicitud.fechaFin().valor();

        Programacion programacion = transacciones.ejecutar(() -> {
            Vacaciones actual = consultar(id);
            if (solicitud.empleadoId() != null && !solicitud.empleadoId().equals(actual.empleadoId())) {
                throw new SolicitudInvalidaException("empleadoId", "No se puede cambiar el empleado del período");
            }
            if (!actual.esReprogramable(ahora)) {
                throw new ConflictoDeEstadoException("El período " + id + " no se puede reprogramar: está "
                        + actual.estado() + (actual.estaVigente() ? " y ya inició" : ""));
            }
            ReglasVacaciones.validarFechas(fechaInicio, fechaFin, ahora);
            EmpleadoReplica empleado = empleadoHabilitado(actual.empleadoId());
            vacaciones.buscarOtroVigenteSolapado(id, actual.empleadoId(), fechaInicio, fechaFin)
                    .ifPresent(ServicioVacaciones::rechazarSolapamiento);
            if (!vacaciones.reprogramar(id, fechaInicio, fechaFin)) {
                throw new ConflictoDeEstadoException("El período " + id + " cambió de estado y no se puede reprogramar");
            }
            return new Programacion(actual.conFechas(fechaInicio, fechaFin), empleado.email());
        });

        // Después del commit, como en programar.
        Vacaciones reprogramada = programacion.vacaciones();
        publicador.publicar(VacacionesProgramadas.TIPO,
                VacacionesProgramadas.de(reprogramada, programacion.email(), reloj.getZone()));
        return reprogramada;
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
            if (!actual.esCancelable(LocalDateTime.now(reloj))) {
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
