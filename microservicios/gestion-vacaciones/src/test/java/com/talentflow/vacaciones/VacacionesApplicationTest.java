package com.talentflow.vacaciones;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.talentflow.vacaciones.aplicacion.ProcesadorCicloVacaciones;
import com.talentflow.vacaciones.infraestructura.scheduler.VacacionesScheduler;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.context.ApplicationContext;
import org.springframework.test.web.servlet.MockMvc;

/**
 * El contexto de Spring arranca: todos los beans de la aplicación se pueden construir y
 * conectar (un bean faltante impedía arrancar el servicio aunque las pruebas unitarias pasaran).
 * No prueba la conexión a la BD ni al broker (son perezosas y aquí no se usan), ni el esquema
 * de init.sql, ni el scheduler en ejecución: el cron se desactiva y los consumidores no arrancan.
 */
@SpringBootTest(properties = {
        "vacaciones.cron=-", // el scheduler no corre durante la prueba
        "spring.rabbitmq.listener.simple.auto-startup=false"
})
@AutoConfigureMockMvc
class VacacionesApplicationTest {

    @Autowired
    private ApplicationContext contexto;

    @Autowired
    private MockMvc mvc;

    @Test
    void elContextoArrancaConElSchedulerYSuCicloConectados() {
        assertThat(contexto.getBean(VacacionesScheduler.class)).isNotNull();
        assertThat(contexto.getBean(ProcesadorCicloVacaciones.class)).isNotNull();
    }

    /** El Gateway reenvía con changeOrigin: el Host interno no debe colarse en "servers". */
    @Test
    void laEspecificacionUsaServidorRelativoYDeclaraBearerGlobal() throws Exception {
        mvc.perform(get("/vacaciones/openapi.json").header("Host", "vacaciones-service:8085"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.servers.length()").value(1))
                .andExpect(jsonPath("$.servers[0].url").value("/"))
                .andExpect(jsonPath("$.components.securitySchemes.bearerAuth.type").value("http"))
                .andExpect(jsonPath("$.components.securitySchemes.bearerAuth.scheme").value("bearer"))
                .andExpect(jsonPath("$.components.securitySchemes.bearerAuth.bearerFormat").value("JWT"))
                .andExpect(jsonPath("$.security[0].bearerAuth").isArray());
    }
}
