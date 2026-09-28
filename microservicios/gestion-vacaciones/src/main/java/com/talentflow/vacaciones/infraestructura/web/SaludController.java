package com.talentflow.vacaciones.infraestructura.web;

import java.sql.Connection;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.LinkedHashMap;
import java.util.Map;
import javax.sql.DataSource;
import org.springframework.amqp.rabbit.listener.MessageListenerContainer;
import org.springframework.amqp.rabbit.listener.RabbitListenerEndpointRegistry;
import org.springframework.amqp.rabbit.listener.SimpleMessageListenerContainer;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;
import io.swagger.v3.oas.annotations.Hidden;

/** Salud interna (la usa el healthcheck de Docker y el /health agregado del Gateway). */
@RestController
@Hidden
public class SaludController {

    private static final int TIMEOUT_BD_S = 2;

    private final DataSource dataSource;
    private final RabbitListenerEndpointRegistry consumidores;

    public SaludController(DataSource dataSource, RabbitListenerEndpointRegistry consumidores) {
        this.dataSource = dataSource;
        this.consumidores = consumidores;
    }

    @GetMapping("/health")
    public ResponseEntity<Map<String, Object>> salud() {
        String db = bdResponde() ? "UP" : "DOWN";
        // DOWN solo si la BD propia no responde; sin broker el servicio sigue atendiendo
        // REST (degradado) y el broker se reporta como componente.
        String status = db;
        Map<String, Object> cuerpo = new LinkedHashMap<>();
        cuerpo.put("status", status);
        cuerpo.put("service", "vacaciones-service");
        cuerpo.put("timestamp", Instant.now().truncatedTo(ChronoUnit.SECONDS).toString());
        cuerpo.put("components", Map.of("app", "UP", "db", db, "broker", brokerConectado() ? "UP" : "DOWN"));
        return ResponseEntity.status("UP".equals(status) ? 200 : 503).body(cuerpo);
    }

    private boolean bdResponde() {
        try (Connection conexion = dataSource.getConnection()) {
            return conexion.isValid(TIMEOUT_BD_S);
        } catch (Exception e) {
            return false;
        }
    }

    private boolean brokerConectado() {
        for (MessageListenerContainer contenedor : consumidores.getListenerContainers()) {
            if (contenedor instanceof SimpleMessageListenerContainer simple && simple.getActiveConsumerCount() > 0) {
                return true;
            }
        }
        return false;
    }
}
