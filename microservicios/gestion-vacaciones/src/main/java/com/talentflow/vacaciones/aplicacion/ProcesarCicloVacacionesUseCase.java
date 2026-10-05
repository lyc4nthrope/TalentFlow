package com.talentflow.vacaciones.aplicacion;

import com.talentflow.vacaciones.dominio.EstadoVacaciones;
import com.talentflow.vacaciones.dominio.EventoPublisherPort;
import com.talentflow.vacaciones.dominio.Vacaciones;

import java.time.LocalDateTime;
import java.util.List;

import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class ProcesarCicloVacacionesUseCase {
    private final RepositorioVacaciones vacacionesRepositorio;
    private final EventoPublisherPort eventoPublisherPort;

    public ProcesarCicloVacacionesUseCase(
            RepositorioVacaciones vacacionesRepositorio,
            EventoPublisherPort eventoPublisherPort) {
        this.vacacionesRepositorio = vacacionesRepositorio;
        this.eventoPublisherPort = eventoPublisherPort;
    }

@Transactional
public void ejecutar() {
    LocalDateTime hoy = LocalDateTime.now();

    // 1. Iniciar vacaciones pendientes cuya fechaInicio ya llegó
    List<Vacaciones> porIniciar = vacacionesRepositorio
            .findByEstadoAndFechaInicioLessThanEqual(EstadoVacaciones.PROGRAMADA, hoy);

    for (Vacaciones v : porIniciar) {
        // Se genera la nueva instancia con estado EN_CURSO
        Vacaciones vacacionesEnCurso = v.conEstado(EstadoVacaciones.EN_CURSO);
        vacacionesRepositorio.actualizar(vacacionesEnCurso);

        // Publicar evento 'vacaciones.iniciadas'
        eventoPublisherPort.publicarVacacionesIniciadas(vacacionesEnCurso.empleadoId(), vacacionesEnCurso.id());
    }

    // 2. Finalizar vacaciones cuya fechaFin ya pasó
    List<Vacaciones> porFinalizar = vacacionesRepositorio
            .findByEstadoAndFechaFinLessThanEqual(EstadoVacaciones.EN_CURSO, hoy);

    for (Vacaciones v : porFinalizar) {
        // Se genera la nueva instancia con estado FINALIZADA
        Vacaciones vacacionesFinalizadas = v.conEstado(EstadoVacaciones.FINALIZADA);
        vacacionesRepositorio.actualizar(vacacionesFinalizadas);

        // Publicar evento 'vacaciones.finalizadas'
        eventoPublisherPort.publicarVacacionesFinalizadas(vacacionesFinalizadas.empleadoId(), vacacionesFinalizadas.id());
    }
}
}
