package com.talentflow.vacaciones.infraestructura.web;

import io.micrometer.observation.ObservationPredicate;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.http.server.observation.ServerRequestObservationContext;

/** Trazas (Reto 8): sin spans para el healthcheck de Docker ni los scrapes de Prometheus. */
@Configuration
public class ConfiguracionTrazas {

    @Bean
    public ObservationPredicate sinHealthNiActuator() {
        return (nombre, contexto) -> !(contexto instanceof ServerRequestObservationContext servidor
                && esRuido(servidor.getCarrier().getRequestURI()));
    }

    static boolean esRuido(String uri) {
        return "/health".equals(uri) || uri.startsWith("/actuator");
    }
}
