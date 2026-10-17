package com.talentflow.vacaciones.aplicacion;

/**
 * Puerto de publicación de eventos. Contrato: nunca lanza. Devuelve true si el broker
 * confirmó el mensaje; false si no se pudo publicar (el error queda en el log).
 */
public interface PublicadorEventos {

    boolean publicar(String tipo, Object data);
}
