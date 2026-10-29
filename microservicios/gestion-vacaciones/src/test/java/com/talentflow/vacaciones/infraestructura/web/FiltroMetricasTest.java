package com.talentflow.vacaciones.infraestructura.web;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import io.micrometer.prometheusmetrics.PrometheusConfig;
import io.micrometer.prometheusmetrics.PrometheusMeterRegistry;
import jakarta.servlet.ServletException;
import org.junit.jupiter.api.Test;
import org.springframework.mock.web.MockFilterChain;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.mock.web.MockHttpServletResponse;
import org.springframework.web.servlet.HandlerMapping;

class FiltroMetricasTest {

    private final PrometheusMeterRegistry registro = new PrometheusMeterRegistry(PrometheusConfig.DEFAULT);
    private final FiltroMetricas filtro = new FiltroMetricas(registro);

    private void pedir(String metodo, String uri, Object patron, int estado) throws Exception {
        MockHttpServletRequest peticion = new MockHttpServletRequest(metodo, uri);
        MockHttpServletResponse respuesta = new MockHttpServletResponse();
        filtro.doFilter(peticion, respuesta, (req, res) -> {
            if (patron != null) {
                req.setAttribute(HandlerMapping.BEST_MATCHING_PATTERN_ATTRIBUTE, patron);
            }
            ((MockHttpServletResponse) res).setStatus(estado);
        });
    }

    @Test
    void cuentaPorPatronMetodoYEstado() throws Exception {
        pedir("GET", "/vacaciones/1", "/vacaciones/{id}", 200);
        pedir("GET", "/vacaciones/2", "/vacaciones/{id}", 200);
        pedir("GET", "/no-existe", "/**", 404);
        pedir("GET", "/sin-patron", null, 404);
        pedir("GET", "/actuator/prometheus", null, 200);

        String texto = registro.scrape();
        assertThat(texto)
                .contains("http_requests_total{method=\"GET\",route=\"/vacaciones/{id}\",status=\"200\"} 2")
                .contains("http_requests_total{method=\"GET\",route=\"other\",status=\"404\"} 2")
                .contains("http_request_duration_seconds_bucket{method=\"GET\",route=\"/vacaciones/{id}\",status=\"200\",le=\"2.0\"} 2")
                .doesNotContain("/actuator");
    }

    @Test
    void unaExcepcionCuentaComo500() {
        MockHttpServletRequest peticion = new MockHttpServletRequest("POST", "/vacaciones");
        MockFilterChain cadena = new MockFilterChain() {
            @Override
            public void doFilter(jakarta.servlet.ServletRequest req, jakarta.servlet.ServletResponse res)
                    throws ServletException {
                throw new ServletException("boom");
            }
        };
        assertThatThrownBy(() -> filtro.doFilter(peticion, new MockHttpServletResponse(), cadena))
                .isInstanceOf(ServletException.class);
        assertThat(registro.scrape()).contains("http_requests_total{method=\"POST\",route=\"other\",status=\"500\"} 1");
    }
}
