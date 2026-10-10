package com.talentflow.notificaciones;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.talentflow.notificaciones.eventos.Envelope;
import com.talentflow.notificaciones.eventos.EventoInvalidoException;
import com.talentflow.notificaciones.eventos.EventoParser;
import java.nio.charset.StandardCharsets;
import org.junit.jupiter.api.Test;

class EventoParserTest {

    private final EventoParser parser = new EventoParser(new ObjectMapper());

    private static byte[] bytes(String json) {
        return json.getBytes(StandardCharsets.UTF_8);
    }

    @Test
    void parseaUnEnvelopeValido() {
        Envelope e = parser.parsear(bytes("""
                {"id":"3f9b2c10","type":"empleado.creado","version":1,"occurredAt":"2026-03-01T14:32:05Z",
                 "producer":"empleados-service","data":{"empleadoId":"E001"}}"""));

        assertThat(e.id()).isEqualTo("3f9b2c10");
        assertThat(e.type()).isEqualTo("empleado.creado");
        assertThat(e.version()).isEqualTo(1);
        assertThat(e.data().get("empleadoId").asText()).isEqualTo("E001");
    }

    @Test
    void rechazaJsonInvalido() {
        assertThatThrownBy(() -> parser.parsear(bytes("esto no es json")))
                .isInstanceOf(EventoInvalidoException.class);
    }

    @Test
    void rechazaUnMensajeQueNoEsObjeto() {
        assertThatThrownBy(() -> parser.parsear(bytes("[1,2,3]")))
                .isInstanceOf(EventoInvalidoException.class);
    }

    @Test
    void rechazaEnvelopeSinCamposObligatorios() {
        assertThatThrownBy(() -> parser.parsear(bytes("{\"type\":\"empleado.creado\"}")))
                .isInstanceOf(EventoInvalidoException.class)
                .hasMessageContaining("id")
                .hasMessageContaining("data");
    }

    @Test
    void rechazaDataQueNoEsObjeto() {
        assertThatThrownBy(() -> parser.parsear(bytes("""
                {"id":"1","type":"x","version":1,"occurredAt":"t","producer":"p","data":"texto"}""")))
                .isInstanceOf(EventoInvalidoException.class);
    }
}
