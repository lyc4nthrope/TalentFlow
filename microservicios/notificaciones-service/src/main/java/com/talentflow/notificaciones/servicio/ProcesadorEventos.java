package com.talentflow.notificaciones.servicio;

import com.talentflow.notificaciones.dominio.Notificacion;
import com.talentflow.notificaciones.dominio.NotificacionNueva;
import com.talentflow.notificaciones.eventos.Envelope;
import com.talentflow.notificaciones.eventos.EventoParser;
import com.talentflow.notificaciones.repositorio.NotificacionRepository;
import java.util.Optional;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;
import org.springframework.transaction.support.TransactionTemplate;

/**
 * Procesa un mensaje del broker: valida, deduplica por id de mensaje, registra la notificación
 * y "envía" (simula) el correo con un log estructurado.
 *
 * La deduplicación y el registro ocurren en UNA transacción: si el guardado falla, el id del
 * mensaje no queda marcado y la reentrega del broker reintenta; si el mensaje llega duplicado,
 * el INSERT en eventos_procesados no inserta nada y se descarta sin repetir el efecto.
 */
@Service
public class ProcesadorEventos {

    public enum Resultado { PROCESADO, DUPLICADO, IGNORADO }

    private static final Logger log = LoggerFactory.getLogger(ProcesadorEventos.class);

    private final EventoParser parser;
    private final ConstructorNotificaciones constructor;
    private final NotificacionRepository repositorio;
    private final TransactionTemplate transaccion;

    public ProcesadorEventos(EventoParser parser, ConstructorNotificaciones constructor,
                             NotificacionRepository repositorio, TransactionTemplate transaccion) {
        this.parser = parser;
        this.constructor = constructor;
        this.repositorio = repositorio;
        this.transaccion = transaccion;
    }

    public Resultado procesar(byte[] cuerpo) {
        Envelope evento = parser.parsear(cuerpo);

        Optional<NotificacionNueva> nueva = constructor.construir(evento);
        if (nueva.isEmpty()) {
            return Resultado.IGNORADO;
        }

        Notificacion guardada = transaccion.execute(estado -> {
            if (!repositorio.registrarEvento(evento.id())) {
                return null;
            }
            return repositorio.guardar(nueva.get(), evento.id());
        });

        if (guardada == null) {
            log.info("Evento duplicado descartado: type={} id={}", evento.type(), evento.id());
            return Resultado.DUPLICADO;
        }

        // Simulación del envío (se emite DESPUÉS de confirmar la transacción)
        log.info("[NOTIFICACIÓN] Tipo: {} | Para: {} | Mensaje: \"{}\"",
                guardada.tipo().etiquetaLog(), guardada.destinatario(), guardada.mensaje());
        return Resultado.PROCESADO;
    }
}
