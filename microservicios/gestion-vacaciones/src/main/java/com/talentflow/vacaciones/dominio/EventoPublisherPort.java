package com.talentflow.vacaciones.dominio;

public interface EventoPublisherPort {

    void publicarVacacionesIniciadas(String empleadoId, String vacacionesId);

    void publicarVacacionesFinalizadas(String empleadoId, String vacacionesId);
}