package com.talentflow.notificaciones.config;

import org.springframework.amqp.core.Binding;
import org.springframework.amqp.core.BindingBuilder;
import org.springframework.amqp.core.Queue;
import org.springframework.amqp.core.QueueBuilder;
import org.springframework.amqp.core.TopicExchange;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * Topología (compartida por todo el ecosistema, ver docs/eventos.md):
 * exchange topic durable + una cola durable propia enlazada a los eventos de interés.
 * Spring AMQP (RabbitAdmin) la declara automáticamente al conectar.
 */
@Configuration
public class BrokerConfig {

    @Bean
    public TopicExchange eventosExchange(@Value("${app.broker.exchange}") String nombre) {
        return new TopicExchange(nombre, true, false);
    }

    @Bean
    public Queue notificacionesQueue(@Value("${app.broker.queue}") String nombre) {
        return QueueBuilder.durable(nombre).build();
    }

    @Bean
    public Binding bindingEmpleadoCreado(Queue notificacionesQueue, TopicExchange eventosExchange) {
        return BindingBuilder.bind(notificacionesQueue).to(eventosExchange).with("empleado.creado");
    }

    @Bean
    public Binding bindingEmpleadoRetirado(Queue notificacionesQueue, TopicExchange eventosExchange) {
        return BindingBuilder.bind(notificacionesQueue).to(eventosExchange).with("empleado.retirado");
    }

    @Bean
    public Binding bindingVacacionesProgramadas(Queue notificacionesQueue, TopicExchange eventosExchange) {
        return BindingBuilder.bind(notificacionesQueue).to(eventosExchange).with("vacaciones.programadas");
    }
}
