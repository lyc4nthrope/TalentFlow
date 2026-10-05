package com.talentflow.vacaciones;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

/** Servicio de Gestión de Vacaciones (Reto 4): REST para RR. HH. y productor de eventos. */
@SpringBootApplication
@EnableScheduling
public class VacacionesApplication {

    public static void main(String[] args) {
        SpringApplication.run(VacacionesApplication.class, args);
    }
}
