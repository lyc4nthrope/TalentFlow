package com.talentflow.vacaciones.infraestructura.web;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import java.sql.Connection;
import java.util.Map;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicReference;
import javax.sql.DataSource;
import org.junit.jupiter.api.Test;
import org.springframework.amqp.rabbit.listener.RabbitListenerEndpointRegistry;
import org.springframework.http.ResponseEntity;

class SaludControllerTest {

    @Test
    void devuelveDownYMantieneLaInterrupcionCuandoLaBDNoResponde() throws Exception {
        DataSource dataSource = mock(DataSource.class);
        RabbitListenerEndpointRegistry consumidores = mock(RabbitListenerEndpointRegistry.class);
        when(consumidores.getListenerContainers()).thenReturn(java.util.List.of());

        Connection conexion = mock(Connection.class);
        when(dataSource.getConnection()).thenAnswer(invocation -> {
            Thread.sleep(5000L);
            return conexion;
        });

        SaludController controller = new SaludController(dataSource, consumidores);
        AtomicReference<ResponseEntity<Map<String, Object>>> respuesta = new AtomicReference<>();
        AtomicBoolean hiloInterrumpido = new AtomicBoolean(false);

        Thread hilo = new Thread(() -> {
            try {
                respuesta.set(controller.salud());
                hiloInterrumpido.set(Thread.currentThread().isInterrupted());
            } catch (Exception e) {
                throw new RuntimeException(e);
            }
        }, "salud-controller-test");

        hilo.start();
        Thread.sleep(100L);
        hilo.interrupt();
        hilo.join();

        assertThat(hiloInterrumpido.get()).isTrue();
        assertThat(respuesta.get()).isNotNull();
        assertThat(respuesta.get().getStatusCodeValue()).isEqualTo(503);
        assertThat(respuesta.get().getBody()).isNotNull();
        assertThat(respuesta.get().getBody().get("status")).isEqualTo("DOWN");
        assertThat(respuesta.get().getBody().get("checks")).isEqualTo(Map.of("database", "DOWN", "messageBroker", "DOWN"));
    }
}
