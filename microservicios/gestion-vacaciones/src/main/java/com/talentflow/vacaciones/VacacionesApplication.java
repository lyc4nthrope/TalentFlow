package com.talentflow.vacaciones;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

/**
 * Servicio de Gestión de Vacaciones: REST para RR. HH. (Reto 4) y scheduler que publica el inicio y
 * el fin de cada período (Reto 5).
 */
@SpringBootApplication
@EnableScheduling
public class VacacionesApplication {

    public static void main(String[] args) {
        SpringApplication.run(VacacionesApplication.class, args);
    }
}
