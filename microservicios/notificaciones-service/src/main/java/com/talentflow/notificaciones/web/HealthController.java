package com.talentflow.notificaciones.web;

import io.swagger.v3.oas.annotations.Hidden;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.amqp.rabbit.listener.MessageListenerContainer;
import org.springframework.amqp.rabbit.listener.RabbitListenerEndpointRegistry;
import org.springframework.amqp.rabbit.listener.SimpleMessageListenerContainer;
import org.springframework.http.ResponseEntity;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Salud interna (la usa el healthcheck de docker-compose). Mismo formato que los demás servicios.
 * El servicio está DOWN solo si su base de datos no responde; un broker caído es "degradado pero vivo".
 */
@RestController
@Hidden
public class HealthController {

    public static final String ID_LISTENER = "notificaciones-listener";

    private final JdbcTemplate jdbc;
    private final RabbitListenerEndpointRegistry registro;

    public HealthController(JdbcTemplate jdbc, RabbitListenerEndpointRegistry registro) {
        this.jdbc = jdbc;
        this.registro = registro;
    }

    @GetMapping("/health")
    public ResponseEntity<Map<String, Object>> health() {
        boolean dbUp;
        try {
            jdbc.queryForObject("SELECT 1", Integer.class);
            dbUp = true;
        } catch (RuntimeException e) {
            dbUp = false;
        }

        boolean brokerUp = false;
        MessageListenerContainer contenedor = registro.getListenerContainer(ID_LISTENER);
        if (contenedor instanceof SimpleMessageListenerContainer simple) {
            brokerUp = simple.getActiveConsumerCount() > 0;
        }

        Map<String, String> componentes = new LinkedHashMap<>();
        componentes.put("app", "UP");
        componentes.put("db", dbUp ? "UP" : "DOWN");
        componentes.put("broker", brokerUp ? "UP" : "DOWN");

        Map<String, Object> cuerpo = new LinkedHashMap<>();
        cuerpo.put("status", dbUp ? "UP" : "DOWN");
        cuerpo.put("service", "notificaciones-service");
        cuerpo.put("timestamp", Instant.now().truncatedTo(ChronoUnit.SECONDS).toString());
        cuerpo.put("components", componentes);

        return ResponseEntity.status(dbUp ? 200 : 503).body(cuerpo);
    }
}
