package com.talentflow.vacaciones.infraestructura.web;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.talentflow.vacaciones.aplicacion.ServicioVacaciones;
import com.talentflow.vacaciones.dominio.ConflictoDeEstadoException;
import com.talentflow.vacaciones.dominio.EstadoVacaciones;
import com.talentflow.vacaciones.dominio.SolicitudInvalidaException;
import com.talentflow.vacaciones.dominio.Vacaciones;
import com.talentflow.vacaciones.dominio.VacacionesNoEncontradasException;
import java.time.Instant;
import java.time.LocalDate;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.http.MediaType;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest(VacacionesController.class)
class VacacionesControllerTest {

    private static final Vacaciones PERIODO = new Vacaciones("V-2026-0001", "E001", LocalDate.parse("2026-12-14"),
            LocalDate.parse("2026-12-20"), EstadoVacaciones.PROGRAMADA, Instant.parse("2026-09-28T15:00:00Z"));

    @Autowired
    private MockMvc mvc;

    @MockitoBean
    private ServicioVacaciones servicio;

    private org.springframework.test.web.servlet.ResultActions postear(String cuerpo) throws Exception {
        return mvc.perform(post("/vacaciones").contentType(MediaType.APPLICATION_JSON).content(cuerpo));
    }

    @Test
    void programarResponde201ConLocationYLaEstructuraDelReto() throws Exception {
        when(servicio.programar(any())).thenReturn(PERIODO);

        postear("{\"empleadoId\":\"E001\",\"fechaInicio\":\"2026-12-14\",\"fechaFin\":\"2026-12-20\"}")
                .andExpect(status().isCreated())
                .andExpect(header().string("Location", "/vacaciones/V-2026-0001"))
                .andExpect(jsonPath("$.id").value("V-2026-0001"))
                .andExpect(jsonPath("$.fechaInicio").value("2026-12-14"))
                .andExpect(jsonPath("$.estado").value("PROGRAMADA"))
                .andExpect(jsonPath("$.fechaCreacion").value("2026-09-28T15:00:00Z"));
    }

    @Test
    void reglaDeNegocioResponde400ConElPeriodoEnConflicto() throws Exception {
        when(servicio.programar(any())).thenThrow(new SolicitudInvalidaException("fechaInicio", "Se cruza", PERIODO));

        postear("{\"empleadoId\":\"E001\",\"fechaInicio\":\"2026-12-18\",\"fechaFin\":\"2027-01-05\"}")
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.status").value(400))
                .andExpect(jsonPath("$.errors[0].field").value("fechaInicio"))
                .andExpect(jsonPath("$.periodoEnConflicto.id").value("V-2026-0001"));
    }

    @Test
    void camposObligatoriosFaltantes400() throws Exception {
        postear("{\"empleadoId\":\"E001\"}")
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.errors.length()").value(2));
    }

    @Test
    void campoDesconocido400() throws Exception {
        postear("{\"empleadoId\":\"E001\",\"fechaInicio\":\"2026-12-14\",\"fechaFin\":\"2026-12-20\",\"estado\":\"EN_CURSO\"}")
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.errors[0].field").value("estado"));
    }

    @Test
    void fechaInexistente400() throws Exception {
        postear("{\"empleadoId\":\"E001\",\"fechaInicio\":\"2026-02-30\",\"fechaFin\":\"2026-03-05\"}")
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.errors[0].field").value("fechaInicio"));
    }

    @Test
    void consultarInexistente404() throws Exception {
        when(servicio.consultar("V-X")).thenThrow(new VacacionesNoEncontradasException("V-X"));

        mvc.perform(get("/vacaciones/V-X")).andExpect(status().isNotFound()).andExpect(jsonPath("$.status").value(404));
    }

    @Test
    void cancelarYaIniciado409() throws Exception {
        when(servicio.cancelar("V-2026-0001")).thenThrow(new ConflictoDeEstadoException("ya inició"));

        mvc.perform(delete("/vacaciones/V-2026-0001")).andExpect(status().isConflict());
    }

    @Test
    void listarPorEmpleadoUsaElParametro() throws Exception {
        when(servicio.listar("E001")).thenReturn(java.util.List.of(PERIODO));

        mvc.perform(get("/vacaciones").param("empleadoId", "E001"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$[0].empleadoId").value("E001"));
    }
}
