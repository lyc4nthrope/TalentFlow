package com.talentflow.vacaciones.infraestructura.persistencia;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.talentflow.vacaciones.dominio.EstadoVacaciones;
import com.talentflow.vacaciones.dominio.SolicitudInvalidaException;
import com.talentflow.vacaciones.dominio.Vacaciones;
import java.time.Instant;
import java.time.LocalDateTime;
import java.time.ZoneId;
import java.util.Map;
import java.util.Optional;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;
import org.springframework.dao.DataIntegrityViolationException;

/**
 * JdbcRepositorioVacaciones contra PostgreSQL 16 real: SQL, mapeo de columnas, consulta de
 * solapamiento (coherente con la restricción EXCLUDE) y transiciones condicionales.
 */
class JdbcRepositorioVacacionesIT extends IntegracionPostgres {

    private static final ZoneId BOGOTA = ZoneId.of("America/Bogota");
    private static final Instant CREACION = Instant.parse("2026-09-28T15:00:00Z");
    // Período de referencia: 14 al 20 de diciembre con fecha sola (fin a las 23:59:59).
    private static final LocalDateTime INICIO = LocalDateTime.parse("2026-12-14T00:00:00");
    private static final LocalDateTime FIN = LocalDateTime.parse("2026-12-20T23:59:59");

    private JdbcRepositorioVacaciones repositorio;

    @BeforeEach
    void preparar() {
        repositorio = new JdbcRepositorioVacaciones(jdbc, BOGOTA);
    }

    private Vacaciones crear(String empleadoId, String inicio, String fin) {
        return repositorio.crear(empleadoId, LocalDateTime.parse(inicio), LocalDateTime.parse(fin), CREACION);
    }

    private void forzarEstado(String id, EstadoVacaciones estado) {
        jdbc.sql("UPDATE vacaciones SET estado = :estado WHERE id = :id")
                .param("estado", estado.name()).param("id", id).update();
    }

    @Test
    void creaUnPeriodoProgramadoConIdDeLaSecuenciaYLoLeeConTodasSusColumnas() {
        Vacaciones creada = repositorio.crear("E001", INICIO, FIN, CREACION);

        assertThat(creada.id()).matches("V-2026-\\d{4}");
        assertThat(creada.estado()).isEqualTo(EstadoVacaciones.PROGRAMADA);
        assertThat(repositorio.buscarPorId(creada.id())).contains(
                new Vacaciones(creada.id(), "E001", INICIO, FIN, EstadoVacaciones.PROGRAMADA, CREACION));
    }

    @Test
    void cadaPeriodoRecibeUnIdDistintoYConsecutivo() {
        Vacaciones primera = crear("E001", "2026-12-14T00:00", "2026-12-15T00:00");
        Vacaciones segunda = crear("E001", "2026-12-16T00:00", "2026-12-17T00:00");

        long n1 = Long.parseLong(primera.id().substring(7));
        long n2 = Long.parseLong(segunda.id().substring(7));
        assertThat(n2).isEqualTo(n1 + 1);
    }

    /** El año del id es el de la fecha de creación en Bogotá, no en UTC. */
    @Test
    void elAnioDelIdSeTomaEnLaZonaHorariaDelNegocio() {
        Instant anioNuevoEnUtc = Instant.parse("2027-01-01T03:00:00Z"); // 31/12/2026 22:00 en Bogotá

        Vacaciones creada = repositorio.crear("E001", INICIO.plusYears(1), FIN.plusYears(1), anioNuevoEnUtc);

        assertThat(creada.id()).startsWith("V-2026-");
    }

    /**
     * fecha_inicio/fecha_fin (TIMESTAMP) guardan la hora local tal cual; fecha_creacion
     * (TIMESTAMPTZ) guarda el instante. Ninguno depende de la zona horaria de la sesión.
     */
    @Test
    void lasFechasNoDependenDeLaZonaHorariaDeLaSesion() {
        Vacaciones leida = transaccion.execute(estado -> {
            jdbc.sql("SET LOCAL TIME ZONE 'Asia/Tokyo'").update();
            Vacaciones creada = repositorio.crear("E001", INICIO, FIN, CREACION);
            return repositorio.buscarPorId(creada.id()).orElseThrow();
        });

        assertThat(leida.fechaInicio()).isEqualTo(INICIO);
        assertThat(leida.fechaFin()).isEqualTo(FIN);
        assertThat(leida.fechaCreacion()).isEqualTo(CREACION);
        Map<String, Object> crudo = jdbc.sql("""
                SELECT fecha_inicio::text AS inicio, fecha_fin::text AS fin,
                       (fecha_creacion AT TIME ZONE 'UTC')::text AS creacion_utc
                  FROM vacaciones WHERE id = :id
                """).param("id", leida.id()).query().singleRow();
        assertThat(crudo).containsEntry("inicio", "2026-12-14 00:00:00")
                .containsEntry("fin", "2026-12-20 23:59:59")
                .containsEntry("creacion_utc", "2026-09-28 15:00:00");
    }

    @Test
    void buscarPorIdDeUnPeriodoInexistenteDevuelveVacio() {
        assertThat(repositorio.buscarPorId("V-2026-9999")).isEmpty();
    }

    @Test
    void listarPorEmpleadoDevuelveSoloLosSuyosOrdenadosPorFechaDeInicio() {
        Vacaciones diciembre = crear("E001", "2026-12-14T00:00", "2026-12-20T23:59:59");
        Vacaciones octubre = crear("E001", "2026-10-05T00:00", "2026-10-09T23:59:59");
        crear("E002", "2026-11-02T00:00", "2026-11-06T23:59:59");

        assertThat(repositorio.listarPorEmpleado("E001")).extracting(Vacaciones::id)
                .containsExactly(octubre.id(), diciembre.id());
        assertThat(repositorio.listarPorEmpleado("E999")).isEmpty();
    }

    @Test
    void listarDevuelveTodosPorFechaDeCreacionYLuegoPorId() {
        Vacaciones despues = repositorio.crear("E001", INICIO, FIN, CREACION.plusSeconds(60));
        Vacaciones antesA = repositorio.crear("E002", INICIO, FIN, CREACION);
        Vacaciones antesB = repositorio.crear("E003", INICIO, FIN, CREACION);

        assertThat(repositorio.listar()).extracting(Vacaciones::id)
                .containsExactly(antesA.id(), antesB.id(), despues.id());
    }

    /**
     * Tabla de solapamiento contra la BD, con la semántica semiabierta [inicio, fin) de F1. Cada
     * fila comprueba dos cosas: la consulta del código y la restricción EXCLUDE del esquema
     * deciden lo mismo (si divergieran, la BD rechazaría lo que el código aceptó o al revés).
     */
    @ParameterizedTest(name = "{3}")
    @CsvSource(delimiter = '|', textBlock = """
            2026-12-14T00:00    | 2026-12-20T23:59:59 | true  | el mismo período
            2026-12-10T00:00    | 2026-12-25T00:00    | true  | lo contiene
            2026-12-16T00:00    | 2026-12-17T00:00    | true  | está dentro
            2026-12-10T00:00    | 2026-12-14T00:00:01 | true  | termina un segundo después del inicio
            2026-12-20T23:59:58 | 2026-12-22T00:00    | true  | empieza un segundo antes del fin
            2026-12-10T00:00    | 2026-12-14T00:00    | false | termina justo cuando empieza el otro
            2026-12-20T23:59:59 | 2026-12-22T00:00    | false | empieza justo cuando termina el otro
            2026-12-01T00:00    | 2026-12-05T00:00    | false | antes, sin tocarse
            2026-12-21T00:00    | 2026-12-24T00:00    | false | después, sin tocarse
            """)
    void laConsultaDeSolapamientoYLaRestriccionExcludeCoinciden(String inicio, String fin, boolean seCruza,
            String caso) {
        Vacaciones existente = repositorio.crear("E001", INICIO, FIN, CREACION);
        LocalDateTime desde = LocalDateTime.parse(inicio);
        LocalDateTime hasta = LocalDateTime.parse(fin);

        Optional<Vacaciones> solapado = repositorio.buscarVigenteSolapado("E001", desde, hasta);

        if (seCruza) {
            assertThat(solapado).contains(existente);
            assertThatThrownBy(() -> repositorio.crear("E001", desde, hasta, CREACION))
                    .isInstanceOf(SolicitudInvalidaException.class);
        } else {
            assertThat(solapado).isEmpty();
            assertThat(repositorio.crear("E001", desde, hasta, CREACION).estado())
                    .isEqualTo(EstadoVacaciones.PROGRAMADA);
        }
    }

    @ParameterizedTest(name = "un período {0} {1} ocupa las fechas")
    @CsvSource({"PROGRAMADA, sí", "EN_CURSO, sí", "CANCELADA, no", "FINALIZADA, no"})
    void soloLosPeriodosVigentesOcupanFechas(EstadoVacaciones estado, String ocupa) {
        Vacaciones existente = repositorio.crear("E001", INICIO, FIN, CREACION);
        forzarEstado(existente.id(), estado);

        Optional<Vacaciones> solapado = repositorio.buscarVigenteSolapado("E001", INICIO, FIN);

        if (ocupa.equals("sí")) {
            assertThat(solapado).map(Vacaciones::estado).contains(estado);
        } else {
            assertThat(solapado).isEmpty();
            assertThat(repositorio.crear("E001", INICIO, FIN, CREACION).estado())
                    .isEqualTo(EstadoVacaciones.PROGRAMADA);
        }
    }

    @Test
    void elSolapamientoNoMezclaEmpleadosYDevuelveElPrimeroPorFechaDeInicio() {
        crear("E002", "2026-12-14T00:00", "2026-12-20T23:59:59");
        Vacaciones tarde = crear("E001", "2026-12-18T00:00", "2026-12-20T00:00");
        Vacaciones temprano = crear("E001", "2026-12-10T00:00", "2026-12-12T00:00");

        assertThat(repositorio.buscarVigenteSolapado("E001", INICIO.minusDays(10), FIN)).contains(temprano);
        assertThat(repositorio.buscarVigenteSolapado("E001", INICIO, FIN)).contains(tarde);
        assertThat(repositorio.buscarVigenteSolapado("E003", INICIO, FIN)).isEmpty();
    }

    /** La red de seguridad: SQLSTATE 23P01 de la restricción EXCLUDE se traduce a una regla de negocio. */
    @Test
    void siDosSolicitudesPasanLaValidacionLaBdRechazaLaSegundaComoSolicitudInvalida() {
        repositorio.crear("E001", INICIO, FIN, CREACION);

        assertThatThrownBy(() -> crear("E001", "2026-12-16T00:00", "2026-12-22T00:00"))
                .isInstanceOfSatisfying(SolicitudInvalidaException.class,
                        e -> assertThat(e.campo()).isEqualTo("fechaInicio"));
        assertThat(repositorio.listarPorEmpleado("E001")).hasSize(1);
    }

    /** Otra violación del esquema no se disfraza de solapamiento: se propaga tal cual. */
    @Test
    void unFinQueNoEsPosteriorAlInicioLoRechazaElEsquemaSinTraducirlo() {
        assertThatThrownBy(() -> repositorio.crear("E001", INICIO, INICIO, CREACION))
                .isInstanceOf(DataIntegrityViolationException.class)
                .hasMessageContaining("chk_fechas_coherentes");
    }

    @Test
    void cancelarSoloAfectaUnPeriodoProgramadoYUnaSegundaVezNoAplica() {
        Vacaciones programada = repositorio.crear("E001", INICIO, FIN, CREACION);
        Vacaciones enCurso = crear("E002", "2026-12-14T00:00", "2026-12-20T00:00");
        forzarEstado(enCurso.id(), EstadoVacaciones.EN_CURSO);

        assertThat(repositorio.cancelar(programada.id())).isTrue();
        assertThat(repositorio.cancelar(programada.id())).isFalse();
        assertThat(repositorio.cancelar(enCurso.id())).isFalse();
        assertThat(repositorio.cancelar("V-2026-9999")).isFalse();
        assertThat(repositorio.buscarPorId(programada.id())).map(Vacaciones::estado).contains(EstadoVacaciones.CANCELADA);
        assertThat(repositorio.buscarPorId(enCurso.id())).map(Vacaciones::estado).contains(EstadoVacaciones.EN_CURSO);
    }

    @Test
    void listarPorIniciarIncluyeElInicioExactoYSoloPeriodosProgramados() {
        LocalDateTime ahora = LocalDateTime.parse("2026-12-14T08:00:00");
        Vacaciones exacta = crear("E001", "2026-12-14T08:00:00", "2026-12-15T00:00");
        Vacaciones anterior = crear("E002", "2026-12-13T00:00", "2026-12-15T00:00");
        crear("E003", "2026-12-14T08:00:01", "2026-12-15T00:00"); // todavía no
        Vacaciones cancelada = crear("E004", "2026-12-13T00:00", "2026-12-15T00:00");
        forzarEstado(cancelada.id(), EstadoVacaciones.CANCELADA);

        assertThat(repositorio.listarPorIniciar(ahora)).extracting(Vacaciones::id)
                .containsExactly(anterior.id(), exacta.id());
    }

    @Test
    void listarPorFinalizarIncluyeElFinExactoYSoloPeriodosEnCurso() {
        LocalDateTime ahora = LocalDateTime.parse("2026-12-20T23:59:59");
        Vacaciones exacta = crear("E001", "2026-12-14T00:00", "2026-12-20T23:59:59");
        Vacaciones anterior = crear("E002", "2026-12-14T00:00", "2026-12-19T00:00");
        Vacaciones pendiente = crear("E003", "2026-12-14T00:00", "2026-12-21T00:00");
        Vacaciones programada = crear("E004", "2026-12-14T00:00", "2026-12-19T00:00");
        for (Vacaciones v : new Vacaciones[] {exacta, anterior, pendiente}) {
            forzarEstado(v.id(), EstadoVacaciones.EN_CURSO);
        }

        assertThat(repositorio.listarPorFinalizar(ahora)).extracting(Vacaciones::id)
                .containsExactly(anterior.id(), exacta.id())
                .doesNotContain(programada.id());
    }

    @Test
    void cambiarEstadoSoloAplicaSiElPeriodoSigueEnElEstadoEsperado() {
        Vacaciones v = repositorio.crear("E001", INICIO, FIN, CREACION);

        assertThat(repositorio.cambiarEstado(v.id(), EstadoVacaciones.EN_CURSO, EstadoVacaciones.FINALIZADA)).isFalse();
        assertThat(repositorio.cambiarEstado(v.id(), EstadoVacaciones.PROGRAMADA, EstadoVacaciones.EN_CURSO)).isTrue();
        assertThat(repositorio.cambiarEstado(v.id(), EstadoVacaciones.PROGRAMADA, EstadoVacaciones.EN_CURSO)).isFalse();
        assertThat(repositorio.cambiarEstado(v.id(), EstadoVacaciones.EN_CURSO, EstadoVacaciones.FINALIZADA)).isTrue();
        assertThat(repositorio.buscarPorId(v.id())).map(Vacaciones::estado).contains(EstadoVacaciones.FINALIZADA);
    }

    /**
     * Dos pasadas del scheduler a la vez (p. ej. dos réplicas del servicio): la segunda espera el
     * bloqueo de fila de la primera, reevalúa el WHERE estado y no aplica. Una sola publicación.
     */
    @Test
    void dosTransicionesConcurrentesDelMismoPeriodoSoloAplicanUnaVez() throws Exception {
        Vacaciones v = repositorio.crear("E001", INICIO, FIN, CREACION);

        Carrera<Boolean, Boolean> carrera = carrera(
                () -> repositorio.cambiarEstado(v.id(), EstadoVacaciones.PROGRAMADA, EstadoVacaciones.EN_CURSO),
                () -> repositorio.cambiarEstado(v.id(), EstadoVacaciones.PROGRAMADA, EstadoVacaciones.EN_CURSO));

        assertThat(carrera.primera()).isTrue();
        assertThat(carrera.segunda()).isFalse();
        assertThat(repositorio.buscarPorId(v.id())).map(Vacaciones::estado).contains(EstadoVacaciones.EN_CURSO);
    }

    /** RR. HH. cancela mientras el scheduler inicia el período: gana quien llegó primero. */
    @Test
    void unaCancelacionConcurrenteConElInicioNoAplicaSiElInicioGanoPrimero() throws Exception {
        Vacaciones v = repositorio.crear("E001", INICIO, FIN, CREACION);

        Carrera<Boolean, Boolean> carrera = carrera(
                () -> repositorio.cambiarEstado(v.id(), EstadoVacaciones.PROGRAMADA, EstadoVacaciones.EN_CURSO),
                () -> repositorio.cancelar(v.id()));

        assertThat(carrera.primera()).isTrue();
        assertThat(carrera.segunda()).isFalse();
        assertThat(repositorio.buscarPorId(v.id())).map(Vacaciones::estado).contains(EstadoVacaciones.EN_CURSO);
    }
}
