package com.talentflow.notificaciones.dominio;

/** Datos de una notificación aún no persistida (derivada de un evento). */
public record NotificacionNueva(TipoNotificacion tipo, String destinatario, String mensaje, String empleadoId) {
}
