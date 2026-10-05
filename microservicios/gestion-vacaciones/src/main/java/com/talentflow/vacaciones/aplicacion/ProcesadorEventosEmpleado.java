package com.talentflow.vacaciones.aplicacion;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.Set;
import java.util.function.Predicate;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Mantiene la réplica local de empleados a partir de empleado.creado, empleado.actualizado y
 * empleado.retirado (Catálogo de Eventos 3.1 a 3.3). Deduplica por el id del mensaje.
 */
public class ProcesadorEventosEmpleado {

    static final int LONGITUD_MAXIMA_ID = 100; // = columna eventos_procesados.id
    static final Set<String> TIPOS_CONSUMIDOS =
            Set.of("empleado.creado", "empleado.actualizado", "empleado.retirado");

    private static final Logger log = LoggerFactory.getLogger(ProcesadorEventosEmpleado.class);

    private final RepositorioEmpleados empleados;
    private final Deduplicador deduplicador;
    private final Transacciones transacciones;
    private final Predicate<RuntimeException> esRechazoPermanente;
    // Los eventos traen más campos de los que usa la réplica: se ignoran los desconocidos.
    private final ObjectMapper json = new ObjectMapper()
            .findAndRegisterModules()
            .configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false);

    @JsonIgnoreProperties(ignoreUnknown = true)
    record DatosEmpleado(String empleadoId, String email) {
    }

    public ProcesadorEventosEmpleado(RepositorioEmpleados empleados, Deduplicador deduplicador,
            Transacciones transacciones, Predicate<RuntimeException> esRechazoPermanente) {
        this.empleados = empleados;
        this.deduplicador = deduplicador;
        this.transacciones = transacciones;
        this.esRechazoPermanente = esRechazoPermanente;
    }

    public Resultado procesar(byte[] cuerpo) {
        JsonNode sobre;
        try {
            sobre = json.readTree(cuerpo);
        } catch (Exception e) {
            log.error("mensaje descartado: JSON no válido ({})", e.getMessage());
            return Resultado.DESCARTADO;
        }

        String id = texto(sobre, "id");
        String tipo = texto(sobre, "type");
        JsonNode data = sobre.path("data");
        // Todos los campos del envelope son obligatorios (Catálogo de Eventos, sección 2).
        boolean envelopeCompleto = id != null && tipo != null && data.isObject()
                && sobre.path("version").asInt(0) >= 1
                && texto(sobre, "occurredAt") != null
                && texto(sobre, "producer") != null;
        if (!envelopeCompleto || id.length() > LONGITUD_MAXIMA_ID) {
            log.error("mensaje descartado: envelope incompleto o id inválido (id={}, type={})", id, tipo);
            return Resultado.DESCARTADO;
        }

        if (!TIPOS_CONSUMIDOS.contains(tipo)) {
            log.warn("evento ignorado (este servicio no lo consume) eventoId={} tipo={}", id, tipo);
            return Resultado.DESCARTADO;
        }

        DatosEmpleado datos;
        try {
            datos = json.treeToValue(data, DatosEmpleado.class);
        } catch (Exception e) {
            log.error("evento descartado: carga útil inválida eventoId={} tipo={}", id, tipo);
            return Resultado.DESCARTADO;
        }
        if (vacio(datos.empleadoId()) || vacio(datos.email())) {
            log.error("evento descartado: faltan empleadoId/email eventoId={} tipo={}", id, tipo);
            return Resultado.DESCARTADO;
        }
        Runnable efecto = switch (tipo) {
            case "empleado.creado" -> () -> empleados.registrar(datos.empleadoId(), datos.email());
            case "empleado.actualizado" -> () -> empleados.actualizarEmail(datos.empleadoId(), datos.email());
            default -> () -> empleados.marcarRetirado(datos.empleadoId(), datos.email()); // empleado.retirado
        };

        try {
            // Registro del id y efecto en UNA transacción: o quedan ambos o ninguno.
            boolean nuevo = transacciones.ejecutar(() -> {
                if (!deduplicador.registrarSiEsNuevo(id)) {
                    return false;
                }
                efecto.run();
                return true;
            });
            if (!nuevo) {
                log.info("evento duplicado descartado: ya se había procesado este id eventoId={} tipo={}", id, tipo);
                return Resultado.DUPLICADO;
            }
            log.info("réplica de empleados actualizada eventoId={} tipo={} empleadoId={}", id, tipo, datos.empleadoId());
            return Resultado.PROCESADO;
        } catch (RuntimeException e) {
            if (esRechazoPermanente.test(e)) {
                log.error("evento descartado: la BD rechazó sus datos eventoId={} tipo={}: {}", id, tipo, e.getMessage());
                return Resultado.DESCARTADO;
            }
            log.error("no se pudo aplicar el evento; se reintentará eventoId={} tipo={}: {}", id, tipo, e.getMessage());
            return Resultado.ERROR_TRANSITORIO;
        }
    }

    private static String texto(JsonNode nodo, String campo) {
        JsonNode valor = nodo.path(campo);
        return valor.isTextual() && !valor.asText().isBlank() ? valor.asText() : null;
    }

    private static boolean vacio(String valor) {
        return valor == null || valor.isBlank();
    }
}
