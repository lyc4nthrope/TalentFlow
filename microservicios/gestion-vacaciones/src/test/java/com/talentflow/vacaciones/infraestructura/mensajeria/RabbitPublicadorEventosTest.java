package com.talentflow.vacaciones.infraestructura.mensajeria;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.talentflow.vacaciones.aplicacion.VacacionesFinalizadas;
import com.talentflow.vacaciones.aplicacion.VacacionesIniciadas;
import com.talentflow.vacaciones.aplicacion.VacacionesProgramadas;
import com.talentflow.vacaciones.dominio.EstadoVacaciones;
import com.talentflow.vacaciones.dominio.Vacaciones;
import java.time.Clock;
import java.time.Instant;
import java.time.LocalDateTime;
import java.time.ZoneId;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.core.RabbitOperations;
import org.springframework.amqp.rabbit.core.RabbitTemplate;

/** Lo que realmente sale al broker: envelope y carga útil, campo por campo según el catálogo. */
class RabbitPublicadorEventosTest {

    private static final ZoneId BOGOTA = ZoneId.of("America/Bogota");
    private static final Clock RELOJ = Clock.fixed(Instant.parse("2026-10-15T15:00:00Z"), BOGOTA);
    private static final Vacaciones PERIODO = new Vacaciones("V-2026-0042", "E001",
            LocalDateTime.of(2026, 10, 20, 8, 0), LocalDateTime.of(2026, 10, 24, 18, 30),
            EstadoVacaciones.EN_CURSO, Instant.parse("2026-10-15T14:00:00Z"));

    // Configurado como el de Spring Boot (módulos de Java Time, fechas ISO).
    private final ObjectMapper json = new ObjectMapper().findAndRegisterModules()
            .disable(SerializationFeature.WRITE_DATES_AS_TIMESTAMPS);
    private RabbitOperations operaciones;
    private RabbitPublicadorEventos publicador;

    @BeforeEach
    @SuppressWarnings("unchecked")
    void preparar() throws Exception {
        RabbitTemplate rabbit = mock(RabbitTemplate.class);
        operaciones = mock(RabbitOperations.class);
        when(operaciones.waitForConfirms(anyLong())).thenReturn(true);
        when(rabbit.invoke(any(RabbitOperations.OperationsCallback.class))).thenAnswer(invocacion ->
                ((RabbitOperations.OperationsCallback<Object>) invocacion.getArgument(0)).doInRabbit(operaciones));
        publicador = new RabbitPublicadorEventos(rabbit, json, "talentflow.eventos", "vacaciones-service", 3000, RELOJ);
    }

    private JsonNode publicar(String tipo, Object data) throws Exception {
        assertThat(publicador.publicar(tipo, data)).isTrue();
        ArgumentCaptor<Message> mensaje = ArgumentCaptor.forClass(Message.class);
        // Routing key = tipo del evento.
        verify(operaciones).send(eq("talentflow.eventos"), eq(tipo), mensaje.capture());
        JsonNode sobre = json.readTree(mensaje.getValue().getBody());
        assertThat(mensaje.getValue().getMessageProperties().getMessageId()).isEqualTo(sobre.get("id").asText());
        return sobre;
    }

    private static List<String> campos(JsonNode nodo) {
        List<String> nombres = new ArrayList<>();
        for (Iterator<String> it = nodo.fieldNames(); it.hasNext();) {
            nombres.add(it.next());
        }
        return nombres;
    }

    @Test
    void elEnvelopeEsElDelCatalogo() throws Exception {
        JsonNode sobre = publicar(VacacionesIniciadas.TIPO, VacacionesIniciadas.de(PERIODO, "juan@empresa.com", BOGOTA));

        assertThat(campos(sobre)).containsExactly("id", "type", "version", "occurredAt", "producer", "data");
        assertThat(sobre.get("id").asText()).matches("[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}");
        assertThat(sobre.get("type").asText()).isEqualTo("vacaciones.iniciadas");
        assertThat(sobre.get("version").asInt()).isEqualTo(1);
        assertThat(sobre.get("occurredAt").asText()).isEqualTo("2026-10-15T15:00:00Z");
        assertThat(sobre.get("producer").asText()).isEqualTo("vacaciones-service");
    }

    @Test
    void vacacionesIniciadasLlevaExactamenteLosCamposDel39ConDesfase() throws Exception {
        JsonNode data = publicar(VacacionesIniciadas.TIPO,
                VacacionesIniciadas.de(PERIODO, "juan@empresa.com", BOGOTA)).get("data");

        assertThat(campos(data)).containsExactly("vacacionesId", "empleadoId", "email", "fechaInicio", "fechaFin");
        assertThat(data.get("vacacionesId").asText()).isEqualTo("V-2026-0042");
        assertThat(data.get("email").asText()).isEqualTo("juan@empresa.com");
        assertThat(data.get("fechaInicio").asText()).isEqualTo("2026-10-20T08:00:00-05:00");
        assertThat(data.get("fechaFin").asText()).isEqualTo("2026-10-24T18:30:00-05:00");
    }

    @Test
    void vacacionesFinalizadasLlevaExactamenteLosCamposDel310() throws Exception {
        JsonNode data = publicar(VacacionesFinalizadas.TIPO,
                VacacionesFinalizadas.de(PERIODO, "juan@empresa.com", BOGOTA)).get("data");

        assertThat(campos(data)).containsExactly("vacacionesId", "empleadoId", "email", "fechaFin");
        assertThat(data.get("fechaFin").asText()).isEqualTo("2026-10-24T18:30:00-05:00");
    }

    @Test
    void vacacionesProgramadasLlevaExactamenteLosCamposDel38() throws Exception {
        JsonNode data = publicar(VacacionesProgramadas.TIPO,
                VacacionesProgramadas.de(PERIODO, "juan@empresa.com", BOGOTA)).get("data");

        assertThat(campos(data)).containsExactly("vacacionesId", "empleadoId", "email", "fechaInicio", "fechaFin", "diasHabiles");
        assertThat(data.get("fechaInicio").asText()).isEqualTo("2026-10-20T08:00:00-05:00");
        assertThat(data.get("diasHabiles").asInt()).isEqualTo(4); // martes 20 a sábado 24: mar-vie
    }

    @Test
    void siElBrokerNoConfirmaDevuelveFalseSinLanzar() throws Exception {
        when(operaciones.waitForConfirms(anyLong())).thenReturn(false);

        assertThat(publicador.publicar(VacacionesFinalizadas.TIPO,
                VacacionesFinalizadas.de(PERIODO, "juan@empresa.com", BOGOTA))).isFalse();
    }
}
