package com.talentflow.notificaciones.web;

import jakarta.servlet.http.HttpServletRequest;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.LinkedHashMap;
import java.util.Map;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.ErrorResponse;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

/** Errores con el mismo formato que el resto del sistema: { status, error, message, timestamp, path }. */
@RestControllerAdvice
public class ManejadorErrores {

    private static final Logger log = LoggerFactory.getLogger(ManejadorErrores.class);

    @ExceptionHandler(Exception.class)
    public ResponseEntity<Map<String, Object>> manejar(Exception e, HttpServletRequest request) {
        // Las excepciones propias de Spring MVC (ruta inexistente -> 404, método no permitido -> 405,
        // parámetro mal formado -> 400...) ya traen su código HTTP: se respeta.
        if (e instanceof ErrorResponse errorResponse) {
            HttpStatus estado = HttpStatus.resolve(errorResponse.getStatusCode().value());
            if (estado != null && estado.is4xxClientError()) {
                String mensaje = estado == HttpStatus.NOT_FOUND ? "Recurso no encontrado" : estado.getReasonPhrase();
                return respuesta(estado, mensaje, request);
            }
        }
        log.error("Error no controlado en {}", request.getRequestURI(), e);
        return respuesta(HttpStatus.INTERNAL_SERVER_ERROR, "Error interno del servidor", request);
    }

    private static ResponseEntity<Map<String, Object>> respuesta(HttpStatus estado, String mensaje,
                                                                 HttpServletRequest request) {
        Map<String, Object> cuerpo = new LinkedHashMap<>();
        cuerpo.put("status", estado.value());
        cuerpo.put("error", estado.getReasonPhrase());
        cuerpo.put("message", mensaje);
        cuerpo.put("timestamp", Instant.now().truncatedTo(ChronoUnit.SECONDS).toString());
        cuerpo.put("path", request.getRequestURI());
        return ResponseEntity.status(estado).body(cuerpo);
    }
}
