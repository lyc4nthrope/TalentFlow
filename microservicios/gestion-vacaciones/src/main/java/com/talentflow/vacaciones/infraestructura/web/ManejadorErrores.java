package com.talentflow.vacaciones.infraestructura.web;

import com.fasterxml.jackson.databind.exc.InvalidFormatException;
import com.fasterxml.jackson.databind.exc.UnrecognizedPropertyException;
import com.talentflow.vacaciones.dominio.ConflictoDeEstadoException;
import com.talentflow.vacaciones.dominio.SolicitudInvalidaException;
import com.talentflow.vacaciones.dominio.VacacionesNoEncontradasException;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.web.HttpRequestMethodNotSupportedException;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;
import org.springframework.web.servlet.resource.NoResourceFoundException;

/** Traduce las excepciones al formato de error común del ecosistema. */
@RestControllerAdvice
public class ManejadorErrores {

    private static final Logger log = LoggerFactory.getLogger(ManejadorErrores.class);

    @ExceptionHandler(SolicitudInvalidaException.class)
    ResponseEntity<Map<String, Object>> reglaDeNegocio(SolicitudInvalidaException e, HttpServletRequest req) {
        Map<String, Object> cuerpo = cuerpo(HttpStatus.BAD_REQUEST, e.getMessage(), req,
                List.of(Map.of("field", e.campo(), "message", e.getMessage())));
        if (e.periodoEnConflicto() != null) {
            cuerpo.put("periodoEnConflicto", e.periodoEnConflicto()); // validación 3 del reto
        }
        return ResponseEntity.badRequest().body(cuerpo);
    }

    @ExceptionHandler(MethodArgumentNotValidException.class)
    ResponseEntity<Map<String, Object>> validacion(MethodArgumentNotValidException e, HttpServletRequest req) {
        List<Map<String, Object>> errores = e.getBindingResult().getFieldErrors().stream()
                .map(error -> Map.<String, Object>of("field", error.getField(), "message", "Es un campo obligatorio"))
                .toList();
        return ResponseEntity.badRequest().body(cuerpo(HttpStatus.BAD_REQUEST, "La solicitud no es válida", req, errores));
    }

    @ExceptionHandler(HttpMessageNotReadableException.class)
    ResponseEntity<Map<String, Object>> cuerpoIlegible(HttpMessageNotReadableException e, HttpServletRequest req) {
        List<Map<String, Object>> errores = List.of();
        if (e.getCause() instanceof UnrecognizedPropertyException desconocido) {
            // Asignación masiva: campos fuera del contrato se rechazan, no se ignoran.
            errores = List.of(Map.of("field", desconocido.getPropertyName(), "message", "Campo desconocido"));
        } else if (e.getCause() instanceof InvalidFormatException formato && !formato.getPath().isEmpty()) {
            String campo = formato.getPath().get(formato.getPath().size() - 1).getFieldName();
            errores = List.of(Map.of("field", campo, "message", "Debe ser una fecha válida YYYY-MM-DD"));
        }
        return ResponseEntity.badRequest().body(cuerpo(HttpStatus.BAD_REQUEST, "La solicitud no es válida", req, errores));
    }

    @ExceptionHandler(VacacionesNoEncontradasException.class)
    ResponseEntity<Map<String, Object>> noEncontrado(VacacionesNoEncontradasException e, HttpServletRequest req) {
        return ResponseEntity.status(HttpStatus.NOT_FOUND).body(cuerpo(HttpStatus.NOT_FOUND, e.getMessage(), req, List.of()));
    }

    @ExceptionHandler(ConflictoDeEstadoException.class)
    ResponseEntity<Map<String, Object>> conflicto(ConflictoDeEstadoException e, HttpServletRequest req) {
        return ResponseEntity.status(HttpStatus.CONFLICT).body(cuerpo(HttpStatus.CONFLICT, e.getMessage(), req, List.of()));
    }

    @ExceptionHandler(NoResourceFoundException.class)
    ResponseEntity<Map<String, Object>> rutaDesconocida(NoResourceFoundException e, HttpServletRequest req) {
        return ResponseEntity.status(HttpStatus.NOT_FOUND)
                .body(cuerpo(HttpStatus.NOT_FOUND, "Recurso no encontrado", req, List.of()));
    }

    @ExceptionHandler(HttpRequestMethodNotSupportedException.class)
    ResponseEntity<Map<String, Object>> metodo(HttpRequestMethodNotSupportedException e, HttpServletRequest req) {
        return ResponseEntity.status(HttpStatus.METHOD_NOT_ALLOWED)
                .body(cuerpo(HttpStatus.METHOD_NOT_ALLOWED, "Método no soportado", req, List.of()));
    }

    @ExceptionHandler(Exception.class)
    ResponseEntity<Map<String, Object>> inesperado(Exception e, HttpServletRequest req) {
        // El detalle queda en el log; al cliente no se le filtran errores internos.
        log.error("error no controlado en {}", req.getRequestURI(), e);
        return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR)
                .body(cuerpo(HttpStatus.INTERNAL_SERVER_ERROR, "Error interno del servidor", req, List.of()));
    }

    private static Map<String, Object> cuerpo(HttpStatus status, String mensaje, HttpServletRequest req,
            List<Map<String, Object>> errores) {
        Map<String, Object> cuerpo = new LinkedHashMap<>();
        cuerpo.put("status", status.value());
        cuerpo.put("error", status.getReasonPhrase());
        cuerpo.put("message", mensaje);
        cuerpo.put("timestamp", Instant.now().truncatedTo(ChronoUnit.SECONDS).toString());
        cuerpo.put("path", req.getRequestURI());
        if (!errores.isEmpty()) {
            cuerpo.put("errors", errores);
        }
        return cuerpo;
    }
}
