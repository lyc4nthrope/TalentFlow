package com.talentflow.vacaciones.infraestructura.persistencia;

import com.talentflow.vacaciones.aplicacion.RepositorioVacaciones;
import com.talentflow.vacaciones.dominio.EstadoVacaciones;
import com.talentflow.vacaciones.dominio.SolicitudInvalidaException;
import com.talentflow.vacaciones.dominio.Vacaciones;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.time.Instant;
import java.time.LocalDateTime;
import java.time.OffsetDateTime;
import java.time.ZoneId;
import java.time.ZoneOffset;
import java.util.List;
import java.util.Optional;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.jdbc.core.simple.JdbcClient;

public class JdbcRepositorioVacaciones implements RepositorioVacaciones {

    /** SQLSTATE de violación de una restricción EXCLUDE (el solapamiento en BD). */
    private static final String VIOLACION_EXCLUSION = "23P01";
    private static final String COLUMNAS = "id, empleado_id, fecha_inicio, fecha_fin, estado, fecha_creacion";

    private final JdbcClient jdbc;
    private final ZoneId zonaHoraria;

    public JdbcRepositorioVacaciones(JdbcClient jdbc, ZoneId zonaHoraria) {
        this.jdbc = jdbc;
        this.zonaHoraria = zonaHoraria;
    }

    @Override
    public Vacaciones crear(String empleadoId, LocalDateTime fechaInicio, LocalDateTime fechaFin,
            Instant fechaCreacion) {
        // Formato del reto: V-2026-0042 (año de creación + secuencia de la BD, única y
        // sin carreras).
        long secuencia = jdbc.sql("SELECT nextval('vacaciones_seq')").query(Long.class).single();
        String id = "V-%d-%04d".formatted(fechaCreacion.atZone(zonaHoraria).getYear(), secuencia);
        try {
            jdbc.sql("""
                    INSERT INTO vacaciones (id, empleado_id, fecha_inicio, fecha_fin, estado, fecha_creacion)
                    VALUES (:id, :empleadoId, :inicio, :fin, 'PROGRAMADA', :creacion)
                    """)
                    .param("id", id)
                    .param("empleadoId", empleadoId)
                    .param("inicio", fechaInicio)
                    .param("fin", fechaFin)
                    .param("creacion", fechaCreacion.atOffset(ZoneOffset.UTC))
                    .update();
        } catch (DataIntegrityViolationException e) {
            // Red de seguridad: la restricción EXCLUDE de la BD impide el solapamiento aun
            // si
            // dos solicitudes simultáneas pasaran la validación previa.
            if (e.getMostSpecificCause() instanceof SQLException sql && VIOLACION_EXCLUSION.equals(sql.getSQLState())) {
                throw new SolicitudInvalidaException("fechaInicio",
                        "El período se cruza con otro período vigente del empleado");
            }
            throw e;
        }
        return new Vacaciones(id, empleadoId, fechaInicio, fechaFin, EstadoVacaciones.PROGRAMADA, fechaCreacion);
    }

    @Override
    public Optional<Vacaciones> buscarPorId(String id) {
        return jdbc.sql("SELECT " + COLUMNAS + " FROM vacaciones WHERE id = :id")
                .param("id", id)
                .query(JdbcRepositorioVacaciones::aVacaciones)
                .optional();
    }

    @Override
    public List<Vacaciones> listar() {
        return jdbc.sql("SELECT " + COLUMNAS + " FROM vacaciones ORDER BY fecha_creacion, id")
                .query(JdbcRepositorioVacaciones::aVacaciones)
                .list();
    }

    @Override
    public List<Vacaciones> listarPorEmpleado(String empleadoId) {
        return jdbc.sql("SELECT " + COLUMNAS + " FROM vacaciones WHERE empleado_id = :empleadoId ORDER BY fecha_inicio")
                .param("empleadoId", empleadoId)
                .query(JdbcRepositorioVacaciones::aVacaciones)
                .list();
    }

    @Override
    public Optional<Vacaciones> buscarVigenteSolapado(String empleadoId, LocalDateTime fechaInicio,
            LocalDateTime fechaFin) {
        // Rangos cerrados [inicio, fin]: se cruzan si inicioA <= finB y inicioB <=
        // finA.
        return jdbc.sql("SELECT " + COLUMNAS + """
                 FROM vacaciones
                WHERE empleado_id = :empleadoId
                  AND estado IN ('PROGRAMADA', 'EN_CURSO')
                  AND fecha_inicio <= :fin
                  AND fecha_fin >= :inicio
                ORDER BY fecha_inicio
                LIMIT 1
                """)
                .param("empleadoId", empleadoId)
                .param("inicio", fechaInicio)
                .param("fin", fechaFin)
                .query(JdbcRepositorioVacaciones::aVacaciones)
                .optional();
    }

    @Override
    public boolean cancelar(String id) {
        return jdbc.sql("UPDATE vacaciones SET estado = 'CANCELADA' WHERE id = :id AND estado = 'PROGRAMADA'")
                .param("id", id)
                .update() == 1;
    }

    private static Vacaciones aVacaciones(ResultSet fila, int numero) throws SQLException {
        return new Vacaciones(
                fila.getString("id"),
                fila.getString("empleado_id"),
                fila.getObject("fecha_inicio", LocalDateTime.class),
                fila.getObject("fecha_fin", LocalDateTime.class),
                EstadoVacaciones.valueOf(fila.getString("estado")),
                fila.getObject("fecha_creacion", OffsetDateTime.class).toInstant());
    }

    @Override
    public List<Vacaciones> findByEstadoAndFechaInicioLessThanEqual(EstadoVacaciones estado, LocalDateTime fecha) {
        return jdbc.sql("SELECT " + COLUMNAS + """
                 FROM vacaciones
                WHERE estado = :estado
                  AND fecha_inicio <= :fecha
                ORDER BY fecha_inicio
                """)
                .param("estado", estado.name())
                .param("fecha", fecha)
                .query(JdbcRepositorioVacaciones::aVacaciones)
                .list();
    }

    @Override
    public List<Vacaciones> findByEstadoAndFechaFinLessThanEqual(EstadoVacaciones estado, LocalDateTime fecha) {
        return jdbc.sql("SELECT " + COLUMNAS + """
                 FROM vacaciones
                WHERE estado = :estado
                  AND fecha_fin <= :fecha
                ORDER BY fecha_fin
                """)
                .param("estado", estado.name())
                .param("fecha", fecha)
                .query(JdbcRepositorioVacaciones::aVacaciones)
                .list();
    }

    @Override
    public boolean actualizarEstado(String id, EstadoVacaciones nuevoEstado) {
        return jdbc.sql("UPDATE vacaciones SET estado = :nuevoEstado WHERE id = :id")
                .param("nuevoEstado", nuevoEstado.name())
                .param("id", id)
                .update() == 1;
    }

    @Override
    public boolean actualizar(Vacaciones vacaciones) {
        return jdbc.sql("""
                UPDATE vacaciones
                   SET estado = :estado
                 WHERE id = :id
                """)
                .param("estado", vacaciones.estado().name())
                .param("id", vacaciones.id())
                .update() == 1;
    }
}
