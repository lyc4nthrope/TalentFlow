package com.talentflow.notificaciones;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

/**
 * Servicio de Notificaciones (Java + Spring Boot).
 * Puramente reactivo: solo consume eventos del broker; REST es únicamente de consulta.
 */
@SpringBootApplication
public class NotificacionesApplication {

    public static void main(String[] args) {
        SpringApplication.run(NotificacionesApplication.class, args);
    }
}
