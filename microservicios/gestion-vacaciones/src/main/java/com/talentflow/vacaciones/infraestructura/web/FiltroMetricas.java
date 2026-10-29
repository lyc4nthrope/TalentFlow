package com.talentflow.vacaciones.infraestructura.web;

import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.Timer;
import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.time.Duration;
import org.springframework.web.filter.OncePerRequestFilter;
import org.springframework.web.servlet.HandlerMapping;

/**
 * Contrato común de métricas de los 7 servicios (Reto 8, D5): http_requests_total y
 * http_request_duration_seconds por method, route y status. Actuator ya publica
 * http_server_requests_seconds, pero con otros nombres; con estos el dashboard usa una sola
 * consulta para todos. Infraestructura (D7): fuera del umbral de cobertura, con su prueba.
 */
public class FiltroMetricas extends OncePerRequestFilter {

    static final String RUTA_OTRA = "other";
    private static final String PREFIJO_ACTUATOR = "/actuator";
    // Hasta 10 s: el umbral de la alerta de latencia (2 s) cae en un bucket exacto.
    private static final Duration[] BUCKETS = {
        Duration.ofMillis(5), Duration.ofMillis(10), Duration.ofMillis(25), Duration.ofMillis(50),
        Duration.ofMillis(100), Duration.ofMillis(250), Duration.ofMillis(500), Duration.ofSeconds(1),
        Duration.ofSeconds(2), Duration.ofSeconds(5), Duration.ofSeconds(10)
    };

    private final MeterRegistry registro;

    public FiltroMetricas(MeterRegistry registro) {
        this.registro = registro;
    }

    @Override
    protected boolean shouldNotFilter(HttpServletRequest peticion) {
        // Los scrapes de /actuator/prometheus no se cuentan como tráfico.
        return peticion.getRequestURI().startsWith(PREFIJO_ACTUATOR);
    }

    @Override
    protected void doFilterInternal(HttpServletRequest peticion, HttpServletResponse respuesta, FilterChain cadena)
            throws ServletException, IOException {
        long inicio = System.nanoTime();
        boolean fallo = true;
        try {
            cadena.doFilter(peticion, respuesta);
            fallo = false;
        } finally {
            String estado = fallo ? "500" : String.valueOf(respuesta.getStatus());
            String[] etiquetas = {"method", peticion.getMethod(), "route", ruta(peticion), "status", estado};
            Counter.builder("http.requests").description("Peticiones HTTP recibidas")
                    .tags(etiquetas).register(registro).increment();
            Timer.builder("http.request.duration").description("Duración de las peticiones HTTP en segundos")
                    .tags(etiquetas).serviceLevelObjectives(BUCKETS).register(registro)
                    .record(Duration.ofNanos(System.nanoTime() - inicio));
        }
    }

    /**
     * Patrón que casó en Spring MVC (/vacaciones/{id}) y no la URL, para no crear una serie
     * por cada id. Sin patrón, o con el comodín de recursos estáticos, es "other".
     */
    static String ruta(HttpServletRequest peticion) {
        Object patron = peticion.getAttribute(HandlerMapping.BEST_MATCHING_PATTERN_ATTRIBUTE);
        if (patron == null || "/**".equals(patron)) {
            return RUTA_OTRA;
        }
        return patron.toString();
    }
}
