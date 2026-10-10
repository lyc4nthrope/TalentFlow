package com.talentflow.notificaciones.servicio;

import com.fasterxml.jackson.databind.JsonNode;
import com.talentflow.notificaciones.dominio.NotificacionNueva;
import com.talentflow.notificaciones.dominio.TipoNotificacion;
import com.talentflow.notificaciones.eventos.Envelope;
import com.talentflow.notificaciones.eventos.EventoInvalidoException;
import java.util.Optional;
import org.springframework.stereotype.Component;

/** Traduce un evento del catálogo a la notificación que corresponde (función pura, sin I/O). */
@Component
public class ConstructorNotificaciones {

    public static final String EMPLEADO_CREADO = "empleado.creado";
    public static final String EMPLEADO_RETIRADO = "empleado.retirado";
    public static final String VACACIONES_PROGRAMADAS = "vacaciones.programadas";

    /** Vacío si este servicio no genera notificación para ese tipo de evento. */
    public Optional<NotificacionNueva> construir(Envelope evento) {
        return switch (evento.type()) {
            case EMPLEADO_CREADO -> Optional.of(bienvenida(evento));
            case EMPLEADO_RETIRADO -> Optional.of(desvinculacion(evento));
            case VACACIONES_PROGRAMADAS -> Optional.of(vacaciones(evento));
            default -> Optional.empty();
        };
    }

    private NotificacionNueva bienvenida(Envelope e) {
        String empleadoId = requerido(e, "empleadoId");
        String email = requerido(e, "email");
        String nombre = (opcional(e, "nombre") + " " + opcional(e, "apellido")).trim();
        if (nombre.isEmpty()) {
            nombre = email;
        }
        String mensaje = "Bienvenido " + nombre + ". Su registro como empleado fue exitoso.";
        return new NotificacionNueva(TipoNotificacion.BIENVENIDA, email, mensaje, empleadoId);
    }

    private NotificacionNueva desvinculacion(Envelope e) {
        String empleadoId = requerido(e, "empleadoId");
        String email = requerido(e, "email");
        String fecha = opcional(e, "fechaRetiro");
        String motivo = opcional(e, "motivo");
        StringBuilder mensaje = new StringBuilder("Su cuenta ha sido desactivada por su retiro de la empresa");
        if (!fecha.isEmpty()) {
            mensaje.append(" (fecha de retiro: ").append(fecha).append(")");
        }
        mensaje.append(".");
        if (!motivo.isEmpty()) {
            mensaje.append(" Motivo: ").append(motivo).append(".");
        }
        return new NotificacionNueva(TipoNotificacion.DESVINCULACION, email, mensaje.toString(), empleadoId);
    }

    private NotificacionNueva vacaciones(Envelope e) {
        String empleadoId = requerido(e, "empleadoId");
        String email = requerido(e, "email");
        String inicio = requerido(e, "fechaInicio");
        String fin = requerido(e, "fechaFin");
        String periodo = opcional(e, "vacacionesId");
        JsonNode dias = e.data().get("diasHabiles");

        StringBuilder mensaje = new StringBuilder("Sus vacaciones del ")
                .append(inicio).append(" al ").append(fin);
        if (dias != null && dias.isNumber()) {
            mensaje.append(" (").append(dias.asInt()).append(" días hábiles)");
        }
        mensaje.append(" han sido programadas");
        if (!periodo.isEmpty()) {
            mensaje.append(" [período ").append(periodo).append("]");
        }
        mensaje.append(".");
        return new NotificacionNueva(TipoNotificacion.VACACIONES, email, mensaje.toString(), empleadoId);
    }

    private static String requerido(Envelope e, String campo) {
        JsonNode valor = e.data().get(campo);
        if (valor == null || !valor.isTextual() || valor.asText().isBlank()) {
            throw new EventoInvalidoException("data." + campo + " es obligatorio en " + e.type());
        }
        return valor.asText().trim();
    }

    private static String opcional(Envelope e, String campo) {
        JsonNode valor = e.data().get(campo);
        if (valor == null || valor.isNull()) {
            return "";
        }
        return valor.asText().trim();
    }
}
