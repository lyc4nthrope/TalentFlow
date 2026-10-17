package com.talentflow.vacaciones.infraestructura.persistencia;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.sql.SQLException;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;
import org.springframework.dao.DataAccessException;

/**
 * Las restricciones de init.sql existen y funcionan en PostgreSQL 16: se insertan filas
 * directamente (sin pasar por el código) y se comprueba que la BD las rechaza con el SQLSTATE
 * esperado. Es la red de seguridad que el código asume.
 */
class EsquemaVacacionesIT extends IntegracionPostgres {

    private static final String NO_NULO = "23502";
    private static final String UNICO = "23505";
    private static final String CHECK = "23514";
    private static final String EXCLUSION = "23P01";
    private static final String DEMASIADO_LARGO = "22001";

    private void insertar(String id, String empleadoId, String inicio, String fin, String estado) {
        jdbc.sql("""
                INSERT INTO vacaciones (id, empleado_id, fecha_inicio, fecha_fin, estado, fecha_creacion)
                VALUES (:id, :empleadoId, CAST(:inicio AS timestamp), CAST(:fin AS timestamp), :estado, now())
                """)
                .param("id", id).param("empleadoId", empleadoId)
                .param("inicio", inicio).param("fin", fin).param("estado", estado)
                .update();
    }

    private static String sqlState(Throwable error) {
        return error instanceof DataAccessException acceso && acceso.getMostSpecificCause() instanceof SQLException sql
                ? sql.getSQLState() : null;
    }

    private void rechaza(Runnable operacion, String sqlStateEsperado, String restriccion) {
        assertThatThrownBy(operacion::run)
                .isInstanceOf(DataAccessException.class)
                .satisfies(e -> assertThat(sqlState(e)).isEqualTo(sqlStateEsperado))
                .hasMessageContaining(restriccion);
    }

    @Test
    void unPeriodoValidoSeAcepta() {
        insertar("V-1", "E001", "2026-12-14 00:00", "2026-12-20 23:59:59", "PROGRAMADA");

        assertThat(jdbc.sql("SELECT count(*) FROM vacaciones").query(Long.class).single()).isEqualTo(1);
    }

    @ParameterizedTest(name = "rechaza {0} nulo")
    @CsvSource({"empleado_id", "fecha_inicio", "fecha_fin", "estado", "fecha_creacion"})
    void lasColumnasObligatoriasDeVacacionesNoAceptanNulos(String columna) {
        rechaza(() -> jdbc.sql("""
                INSERT INTO vacaciones (id, empleado_id, fecha_inicio, fecha_fin, estado, fecha_creacion)
                VALUES ('V-1', 'E001', '2026-12-14', '2026-12-20', 'PROGRAMADA', now())
                """.replace(valorDe(columna), "NULL")).update(), NO_NULO, columna);
    }

    private static String valorDe(String columna) {
        return switch (columna) {
            case "empleado_id" -> "'E001'";
            case "fecha_inicio" -> "'2026-12-14'";
            case "fecha_fin" -> "'2026-12-20'";
            case "estado" -> "'PROGRAMADA'";
            default -> "now()";
        };
    }

    @Test
    void elIdDelPeriodoEsUnico() {
        insertar("V-1", "E001", "2026-10-01", "2026-10-02", "PROGRAMADA");

        rechaza(() -> insertar("V-1", "E002", "2026-11-01", "2026-11-02", "PROGRAMADA"), UNICO, "vacaciones_pkey");
    }

    @Test
    void elEstadoSoloAceptaLosCuatroDelDominio() {
        rechaza(() -> insertar("V-1", "E001", "2026-10-01", "2026-10-02", "APROBADA"), CHECK, "vacaciones_estado_check");
    }

    @ParameterizedTest(name = "rechaza fin {1} con inicio {0}")
    @CsvSource({"2026-12-14 00:00, 2026-12-14 00:00", "2026-12-14 00:00, 2026-12-13 23:59:59"})
    void elFinDebeSerPosteriorAlInicio(String inicio, String fin) {
        rechaza(() -> insertar("V-1", "E001", inicio, fin, "PROGRAMADA"), CHECK, "chk_fechas_coherentes");
    }

    @Test
    void dosPeriodosVigentesQueSeCruzanSonRechazadosAunEnCurso() {
        insertar("V-1", "E001", "2026-12-14 00:00", "2026-12-20 23:59:59", "EN_CURSO");

        rechaza(() -> insertar("V-2", "E001", "2026-12-20 00:00", "2026-12-22 00:00", "PROGRAMADA"),
                EXCLUSION, "excl_sin_solapamiento");
    }

    @Test
    void laExclusionPermiteRangosContiguosOtrosEmpleadosYPeriodosNoVigentes() {
        insertar("V-1", "E001", "2026-12-14 00:00", "2026-12-20 23:59:59", "PROGRAMADA");

        insertar("V-2", "E001", "2026-12-20 23:59:59", "2026-12-22 00:00", "PROGRAMADA"); // [) contiguo
        insertar("V-3", "E002", "2026-12-14 00:00", "2026-12-20 23:59:59", "PROGRAMADA");
        insertar("V-4", "E001", "2026-12-14 00:00", "2026-12-20 23:59:59", "CANCELADA");
        insertar("V-5", "E001", "2026-12-14 00:00", "2026-12-20 23:59:59", "FINALIZADA");

        assertThat(jdbc.sql("SELECT count(*) FROM vacaciones").query(Long.class).single()).isEqualTo(5);
    }

    /** La restricción también cubre UPDATE: reactivar un cancelado que ya se cruza con otro falla. */
    @Test
    void reactivarUnPeriodoCanceladoQueSeCruzaEsRechazado() {
        insertar("V-1", "E001", "2026-12-14 00:00", "2026-12-20 23:59:59", "CANCELADA");
        insertar("V-2", "E001", "2026-12-16 00:00", "2026-12-18 00:00", "PROGRAMADA");

        rechaza(() -> jdbc.sql("UPDATE vacaciones SET estado = 'PROGRAMADA' WHERE id = 'V-1'").update(),
                EXCLUSION, "excl_sin_solapamiento");
    }

    @Test
    void laReplicaDeEmpleadosExigeEmailYUnIdUnicoDeVeinteCaracteres() {
        jdbc.sql("INSERT INTO empleados_replica (empleado_id, email) VALUES ('E001', 'juan@empresa.com')").update();

        rechaza(() -> jdbc.sql("INSERT INTO empleados_replica (empleado_id, email) VALUES ('E001', 'otro@empresa.com')")
                .update(), UNICO, "empleados_replica_pkey");
        rechaza(() -> jdbc.sql("INSERT INTO empleados_replica (empleado_id, email) VALUES ('E002', NULL)").update(),
                NO_NULO, "email");
        rechaza(() -> jdbc.sql("INSERT INTO empleados_replica (empleado_id, email) VALUES (:id, 'a@b.co')")
                .param("id", "E".repeat(21)).update(), DEMASIADO_LARGO, "character varying(20)");
    }

    @Test
    void elIdDeUnEventoProcesadoEsUnico() {
        jdbc.sql("INSERT INTO eventos_procesados (id) VALUES ('evt-1')").update();

        rechaza(() -> jdbc.sql("INSERT INTO eventos_procesados (id) VALUES ('evt-1')").update(),
                UNICO, "eventos_procesados_pkey");
    }
}
