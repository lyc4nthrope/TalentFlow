package com.talentflow.notificaciones;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.talentflow.notificaciones.dominio.NotificacionNueva;
import com.talentflow.notificaciones.dominio.TipoNotificacion;
import com.talentflow.notificaciones.eventos.Envelope;
import com.talentflow.notificaciones.eventos.EventoInvalidoException;
import com.talentflow.notificaciones.eventos.EventoParser;
import com.talentflow.notificaciones.servicio.ConstructorNotificaciones;
import java.nio.charset.StandardCharsets;
import java.util.Optional;
import org.junit.jupiter.api.Test;

class ConstructorNotificacionesTest {

    private final EventoParser parser = new EventoParser(new ObjectMapper());
    private final ConstructorNotificaciones constructor = new ConstructorNotificaciones();

    private Envelope evento(String tipo, String data) {
        String json = "{\"id\":\"m-1\",\"type\":\"" + tipo + "\",\"version\":1,"
                + "\"occurredAt\":\"2026-10-10T12:00:00Z\",\"producer\":\"p\",\"data\":" + data + "}";
        return parser.parsear(json.getBytes(StandardCharsets.UTF_8));
    }

    @Test
    void empleadoCreadoGeneraBienvenida() {
        NotificacionNueva n = constructor.construir(evento("empleado.creado",
                "{\"empleadoId\":\"E001\",\"nombre\":\"Juan\",\"apellido\":\"Pérez\",\"email\":\"juan@empresa.com\"}"))
                .orElseThrow();

        assertThat(n.tipo()).isEqualTo(TipoNotificacion.BIENVENIDA);
        assertThat(n.destinatario()).isEqualTo("juan@empresa.com");
        assertThat(n.empleadoId()).isEqualTo("E001");
        assertThat(n.mensaje()).startsWith("Bienvenido Juan Pérez");
    }

    @Test
    void empleadoRetiradoGeneraDesvinculacion() {
        NotificacionNueva n = constructor.construir(evento("empleado.retirado",
                "{\"empleadoId\":\"E001\",\"email\":\"juan@empresa.com\","
                        + "\"fechaRetiro\":\"2026-11-30T16:45:00Z\",\"motivo\":\"RENUNCIA\"}"))
                .orElseThrow();

        assertThat(n.tipo()).isEqualTo(TipoNotificacion.DESVINCULACION);
        assertThat(n.mensaje()).startsWith("Su cuenta ha sido desactivada").contains("RENUNCIA");
    }

    @Test
    void vacacionesProgramadasGeneraConfirmacionConLasFechas() {
        NotificacionNueva n = constructor.construir(evento("vacaciones.programadas",
                "{\"vacacionesId\":\"V-2026-0042\",\"empleadoId\":\"E001\",\"email\":\"juan@empresa.com\","
                        + "\"fechaInicio\":\"2027-03-15\",\"fechaFin\":\"2027-03-30\",\"diasHabiles\":12}"))
                .orElseThrow();

        assertThat(n.tipo()).isEqualTo(TipoNotificacion.VACACIONES);
        assertThat(n.mensaje()).startsWith("Sus vacaciones del 2027-03-15 al 2027-03-30")
                .contains("12 días hábiles")
                .contains("V-2026-0042");
    }

    @Test
    void unEventoAjenoNoGeneraNotificacion() {
        Optional<NotificacionNueva> n = constructor.construir(evento("empleado.actualizado", "{\"empleadoId\":\"E001\"}"));
        assertThat(n).isEmpty();
    }

    @Test
    void faltaElEmailEsEventoInvalido() {
        assertThatThrownBy(() -> constructor.construir(evento("empleado.creado", "{\"empleadoId\":\"E001\"}")))
                .isInstanceOf(EventoInvalidoException.class)
                .hasMessageContaining("data.email");
    }

    @Test
    void faltaElEmpleadoIdEsEventoInvalido() {
        assertThatThrownBy(() -> constructor.construir(evento("vacaciones.programadas",
                "{\"email\":\"a@a.com\",\"fechaInicio\":\"2027-01-01\",\"fechaFin\":\"2027-01-05\"}")))
                .isInstanceOf(EventoInvalidoException.class)
                .hasMessageContaining("data.empleadoId");
    }
}
