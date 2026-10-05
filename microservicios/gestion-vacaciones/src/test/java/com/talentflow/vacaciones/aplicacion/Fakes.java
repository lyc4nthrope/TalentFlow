package com.talentflow.vacaciones.aplicacion;

import com.talentflow.vacaciones.dominio.EmpleadoReplica;
import com.talentflow.vacaciones.dominio.EstadoVacaciones;
import com.talentflow.vacaciones.dominio.ReglasVacaciones;
import com.talentflow.vacaciones.dominio.Vacaciones;
import java.time.Instant;
import java.time.LocalDate;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.function.Supplier;

/** Implementaciones en memoria de los puertos, con el comportamiento observable de las reales. */
final class Fakes {

    private Fakes() {
    }

    static final class Vacacionesmem implements RepositorioVacaciones {
        final Map<String, Vacaciones> datos = new LinkedHashMap<>();
        private int secuencia;

        @Override
        public Vacaciones crear(String empleadoId, LocalDate inicio, LocalDate fin, Instant creacion) {
            String id = "V-2026-%04d".formatted(++secuencia);
            Vacaciones v = new Vacaciones(id, empleadoId, inicio, fin, EstadoVacaciones.PROGRAMADA, creacion);
            datos.put(id, v);
            return v;
        }

        @Override
        public Optional<Vacaciones> buscarPorId(String id) {
            return Optional.ofNullable(datos.get(id));
        }

        @Override
        public List<Vacaciones> listar() {
            return new ArrayList<>(datos.values());
        }

        @Override
        public List<Vacaciones> listarPorEmpleado(String empleadoId) {
            return datos.values().stream().filter(v -> v.empleadoId().equals(empleadoId)).toList();
        }

        @Override
        public Optional<Vacaciones> buscarVigenteSolapado(String empleadoId, LocalDate inicio, LocalDate fin) {
            return datos.values().stream()
                    .filter(v -> v.empleadoId().equals(empleadoId) && v.estaVigente())
                    .filter(v -> ReglasVacaciones.seSolapan(v.fechaInicio(), v.fechaFin(), inicio, fin))
                    .findFirst();
        }

        @Override
        public boolean cancelar(String id) {
            Vacaciones v = datos.get(id);
            if (v == null || v.estado() != EstadoVacaciones.PROGRAMADA) {
                return false;
            }
            datos.put(id, v.cancelada());
            return true;
        }
    }

    static final class EmpleadosMem implements RepositorioEmpleados {
        final Map<String, EmpleadoReplica> datos = new LinkedHashMap<>();

        @Override
        public Optional<EmpleadoReplica> buscarParaProgramar(String empleadoId) {
            return Optional.ofNullable(datos.get(empleadoId));
        }

        @Override
        public void registrar(String empleadoId, String email) {
            datos.putIfAbsent(empleadoId, new EmpleadoReplica(empleadoId, email, false));
        }

        @Override
        public void actualizarEmail(String empleadoId, String email) {
            EmpleadoReplica actual = datos.get(empleadoId);
            datos.put(empleadoId, new EmpleadoReplica(empleadoId, email, actual != null && actual.retirado()));
        }

        @Override
        public void marcarRetirado(String empleadoId, String email) {
            EmpleadoReplica actual = datos.get(empleadoId);
            datos.put(empleadoId, new EmpleadoReplica(empleadoId, actual != null ? actual.email() : email, true));
        }
    }

    static final class DeduplicadorMem implements Deduplicador {
        final Set<String> procesados = new HashSet<>();
        RuntimeException fallo;

        @Override
        public boolean registrarSiEsNuevo(String eventoId) {
            if (fallo != null) {
                throw fallo;
            }
            return procesados.add(eventoId);
        }
    }

    static final class PublicadorEspia implements PublicadorEventos {
        record Publicado(String tipo, Object data) {
        }

        final List<Publicado> publicados = new ArrayList<>();
        boolean exito = true;

        @Override
        public boolean publicar(String tipo, Object data) {
            publicados.add(new Publicado(tipo, data));
            return exito;
        }
    }

    /** Sin BD: ejecuta el bloque tal cual (las pruebas de transacción real son de integración). */
    static final class SinTransaccion implements Transacciones {
        @Override
        public <T> T ejecutar(Supplier<T> bloque) {
            return bloque.get();
        }
    }
}
