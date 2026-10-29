package com.talentflow.vacaciones.infraestructura.web;

import static org.assertj.core.api.Assertions.assertThat;

import io.micrometer.observation.Observation;
import io.micrometer.observation.ObservationPredicate;
import org.junit.jupiter.api.Test;
import org.springframework.http.server.observation.ServerRequestObservationContext;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.mock.web.MockHttpServletResponse;

class ConfiguracionTrazasTest {

    private final ObservationPredicate predicado = new ConfiguracionTrazas().sinHealthNiActuator();

    private boolean observa(String uri) {
        var contexto = new ServerRequestObservationContext(
                new MockHttpServletRequest("GET", uri), new MockHttpServletResponse());
        return predicado.test("http.server.requests", contexto);
    }

    @Test
    void omiteHealthYActuatorPeroNoElNegocio() {
        assertThat(observa("/health")).isFalse();
        assertThat(observa("/actuator/prometheus")).isFalse();
        assertThat(observa("/vacaciones/1")).isTrue();
        // Lo que no es una petición HTTP (p. ej. el @Scheduled) se sigue observando.
        assertThat(predicado.test("tasks.scheduled.execution", new Observation.Context())).isTrue();
    }
}
