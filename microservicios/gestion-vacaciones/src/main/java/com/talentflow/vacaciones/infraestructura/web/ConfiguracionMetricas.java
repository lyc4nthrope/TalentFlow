package com.talentflow.vacaciones.infraestructura.web;

import io.micrometer.core.instrument.MeterRegistry;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/** Registra el filtro de métricas (Reto 8, D5) con el registro de Micrometer de Actuator. */
@Configuration
public class ConfiguracionMetricas {

    @Bean
    public FiltroMetricas filtroMetricas(MeterRegistry registro) {
        return new FiltroMetricas(registro);
    }
}
