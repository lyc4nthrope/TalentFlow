package com.talentflow.notificaciones.repositorio;

import com.talentflow.notificaciones.dominio.Notificacion;
import com.talentflow.notificaciones.dominio.NotificacionNueva;
import com.talentflow.notificaciones.dominio.TipoNotificacion;
import java.sql.Timestamp;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.List;
import java.util.UUID;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.stereotype.Repository;

@Repository
public class NotificacionRepository {

    private static final String COLUMNAS = "id, tipo, destinatario, mensaje, fecha_envio, empleado_id";

    private static final RowMapper<Notificacion> MAPPER = (rs, fila) -> new Notificacion(
            rs.getString("id"),
            TipoNotificacion.valueOf(rs.getString("tipo")),
            rs.getString("destinatario"),
            rs.getString("mensaje"),
            rs.getTimestamp("fecha_envio").toInstant(),
            rs.getString("empleado_id"));

    private final JdbcTemplate jdbc;

    public NotificacionRepository(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    /**
     * Registra el id del mensaje. Devuelve true si es nuevo; false si ya se había procesado
     * (mensaje duplicado). Debe llamarse dentro de la misma transacción que guarda la notificación.
     */
    public boolean registrarEvento(String eventoId) {
        int filas = jdbc.update(
                "INSERT INTO eventos_procesados (id) VALUES (?) ON CONFLICT (id) DO NOTHING", eventoId);
        return filas == 1;
    }

    public Notificacion guardar(NotificacionNueva nueva, String eventoId) {
        String id = UUID.randomUUID().toString();
        Instant ahora = Instant.now().truncatedTo(ChronoUnit.SECONDS);
        jdbc.update(
                "INSERT INTO notificaciones (id, tipo, destinatario, mensaje, fecha_envio, empleado_id, evento_id) "
                        + "VALUES (?, ?, ?, ?, ?, ?, ?)",
                id, nueva.tipo().name(), nueva.destinatario(), nueva.mensaje(),
                Timestamp.from(ahora), nueva.empleadoId(), eventoId);
        return new Notificacion(id, nueva.tipo(), nueva.destinatario(), nueva.mensaje(), ahora, nueva.empleadoId());
    }

    public List<Notificacion> listar() {
        return jdbc.query("SELECT " + COLUMNAS + " FROM notificaciones ORDER BY fecha_envio, id", MAPPER);
    }

    public List<Notificacion> listarPorEmpleado(String empleadoId) {
        return jdbc.query(
                "SELECT " + COLUMNAS + " FROM notificaciones WHERE empleado_id = ? ORDER BY fecha_envio, id",
                MAPPER, empleadoId);
    }
}
