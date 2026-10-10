package com.talentflow.notificaciones.config;

import io.swagger.v3.oas.models.OpenAPI;
import io.swagger.v3.oas.models.info.Info;
import io.swagger.v3.oas.models.servers.Server;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class OpenApiConfig {

    @Bean
    public OpenAPI openApi() {
        return new OpenAPI()
                .info(new Info()
                        .title("TalentFlow — Servicio de Notificaciones")
                        .version("1.0.0")
                        .description("Consulta el historial de notificaciones. Se generan solas al consumir "
                                + "empleado.creado (BIENVENIDA), empleado.retirado (DESVINCULACION) y "
                                + "vacaciones.programadas (VACACIONES). El envío del correo es simulado por log. "
                                + "Este servicio no se invoca por REST para crear nada: solo reacciona a eventos."))
                // URL relativa: detrás del Gateway, el Host interno del contenedor no es alcanzable
                // desde el navegador y rompería el "Try it out" de Swagger UI.
                .addServersItem(new Server().url("/").description("API Gateway"));
    }
}
