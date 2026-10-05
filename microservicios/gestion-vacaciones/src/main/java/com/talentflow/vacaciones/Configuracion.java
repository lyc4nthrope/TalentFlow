package com.talentflow.vacaciones;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.talentflow.vacaciones.aplicacion.ProcesadorEventosEmpleado;
import com.talentflow.vacaciones.aplicacion.ServicioVacaciones;
import com.talentflow.vacaciones.aplicacion.Transacciones;
import com.talentflow.vacaciones.infraestructura.mensajeria.ConsumidorEventosEmpleado;
import com.talentflow.vacaciones.infraestructura.mensajeria.RabbitPublicadorEventos;
import com.talentflow.vacaciones.infraestructura.persistencia.JdbcDeduplicador;
import com.talentflow.vacaciones.infraestructura.persistencia.JdbcRepositorioEmpleados;
import com.talentflow.vacaciones.infraestructura.persistencia.JdbcRepositorioVacaciones;
import com.talentflow.vacaciones.infraestructura.persistencia.TransaccionesSpring;
import io.swagger.v3.oas.models.OpenAPI;
import io.swagger.v3.oas.models.info.Info;
import java.time.Clock;
import java.time.ZoneId;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.jdbc.core.simple.JdbcClient;
import org.springframework.transaction.support.TransactionTemplate;

/**
 * Composición de la aplicación: conecta los casos de uso (que no conocen Spring ni la
 * infraestructura) con sus adaptadores de persistencia y mensajería.
 */
@Configuration
public class Configuracion {

    /** Metadatos de la especificación (sin esto springdoc usa "OpenAPI definition"). */
    @Bean
    OpenAPI especificacion() {
        return new OpenAPI().info(new Info()
                .title("TalentFlow - Servicio de Gestión de Vacaciones")
                .version("1.0.0")
                .description("REST para RR. HH.: programa, consulta y cancela períodos de vacaciones con las "
                        + "cuatro validaciones del reto. Mantiene una réplica local de empleados consumiendo "
                        + "empleado.creado/actualizado/retirado y publica vacaciones.programadas (Catálogo de Eventos)."));
    }

    @Bean
    ZoneId zonaHoraria(@Value("${vacaciones.zona-horaria}") String zona) {
        return ZoneId.of(zona);
    }

    /** "Hoy" es el día en la zona horaria del negocio, no en UTC. */
    @Bean
    Clock reloj(ZoneId zonaHoraria) {
        return Clock.system(zonaHoraria);
    }

    @Bean
    Transacciones transacciones(TransactionTemplate plantilla) {
        return new TransaccionesSpring(plantilla);
    }

    @Bean
    JdbcRepositorioVacaciones repositorioVacaciones(JdbcClient jdbc, ZoneId zonaHoraria) {
        return new JdbcRepositorioVacaciones(jdbc, zonaHoraria);
    }

    @Bean
    JdbcRepositorioEmpleados repositorioEmpleados(JdbcClient jdbc) {
        return new JdbcRepositorioEmpleados(jdbc);
    }

    @Bean
    RabbitPublicadorEventos publicadorEventos(RabbitTemplate rabbit, ObjectMapper json, Clock reloj,
            @Value("${vacaciones.eventos.exchange}") String exchange,
            @Value("${vacaciones.eventos.timeout-confirmacion-ms}") long timeoutMs) {
        return new RabbitPublicadorEventos(rabbit, json, exchange, "vacaciones-service", timeoutMs, reloj);
    }

    @Bean
    ServicioVacaciones servicioVacaciones(JdbcRepositorioVacaciones vacaciones, JdbcRepositorioEmpleados empleados,
            RabbitPublicadorEventos publicador, Transacciones transacciones, Clock reloj) {
        return new ServicioVacaciones(vacaciones, empleados, publicador, transacciones, reloj);
    }

    @Bean
    ProcesadorEventosEmpleado procesadorEventosEmpleado(JdbcRepositorioEmpleados empleados, JdbcClient jdbc,
            Transacciones transacciones) {
        // Spring traduce los SQLSTATE 22 (dato inválido) y 23 (restricción violada) a
        // DataIntegrityViolationException: rechazos permanentes, no se reintentan.
        return new ProcesadorEventosEmpleado(empleados, new JdbcDeduplicador(jdbc), transacciones,
                e -> e instanceof DataIntegrityViolationException);
    }

    @Bean
    ConsumidorEventosEmpleado consumidorEventosEmpleado(ProcesadorEventosEmpleado procesador,
            @Value("${vacaciones.eventos.espera-reintento-ms}") long esperaMs) {
        return new ConsumidorEventosEmpleado(procesador, esperaMs);
    }
}
