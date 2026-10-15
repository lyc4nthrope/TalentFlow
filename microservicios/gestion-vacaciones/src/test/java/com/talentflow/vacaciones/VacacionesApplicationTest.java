package com.talentflow.vacaciones;

import static org.assertj.core.api.Assertions.assertThat;

import com.talentflow.vacaciones.aplicacion.ProcesadorCicloVacaciones;
import com.talentflow.vacaciones.infraestructura.scheduler.VacacionesScheduler;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.context.ApplicationContext;

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
class VacacionesApplicationTest {

    @Autowired
    private ApplicationContext contexto;

    @Test
    void elContextoArrancaConElSchedulerYSuCicloConectados() {
        assertThat(contexto.getBean(VacacionesScheduler.class)).isNotNull();
        assertThat(contexto.getBean(ProcesadorCicloVacaciones.class)).isNotNull();
    }
}
