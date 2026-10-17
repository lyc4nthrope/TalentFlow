package com.talentflow.vacaciones.aplicacion;

import com.talentflow.vacaciones.dominio.EmpleadoReplica;
import com.talentflow.vacaciones.dominio.EstadoVacaciones;
import com.talentflow.vacaciones.dominio.ReglasVacaciones;
import com.talentflow.vacaciones.dominio.Vacaciones;
import java.time.Instant;
import java.time.LocalDateTime;
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
        public Vacaciones crear(String empleadoId, LocalDateTime inicio, LocalDateTime fin, Instant creacion) {
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
        public Optional<Vacaciones> buscarVigenteSolapado(String empleadoId, LocalDateTime inicio, LocalDateTime fin) {
            return datos.values().stream()
                    .filter(v -> v.empleadoId().equals(empleadoId) && v.estaVigente())
                    .filter(v -> ReglasVacaciones.seSolapan(v.fechaInicio(), v.fechaFin(), inicio, fin))
                    .findFirst();
        }

        /** Se ejecuta al empezar cancelar: simula otra operación que llega justo antes. */
        Runnable alCancelar = () -> { };

        @Override
        public boolean cancelar(String id) {
            alCancelar.run();
            Vacaciones v = datos.get(id);
            if (v == null || v.estado() != EstadoVacaciones.PROGRAMADA) {
                return false;
            }
            datos.put(id, v.cancelada());
            return true;
        }

        /** Se ejecuta al empezar cambiarEstado: simula otra operación que llega justo antes. */
        Runnable alCambiarEstado = () -> { };
        /** Ids cuyo cambio de estado falla (p. ej. BD caída para ese período). */
        final Set<String> fallarCambioDe = new HashSet<>();

        @Override
        public List<Vacaciones> listarPorIniciar(LocalDateTime ahora) {
            return datos.values().stream()
                    .filter(v -> v.estado() == EstadoVacaciones.PROGRAMADA && !v.fechaInicio().isAfter(ahora))
                    .sorted(java.util.Comparator.comparing(Vacaciones::fechaInicio))
                    .toList();
        }

        @Override
        public List<Vacaciones> listarPorFinalizar(LocalDateTime ahora) {
            return datos.values().stream()
                    .filter(v -> v.estado() == EstadoVacaciones.EN_CURSO && !v.fechaFin().isAfter(ahora))
                    .sorted(java.util.Comparator.comparing(Vacaciones::fechaFin))
                    .toList();
        }

        @Override
        public boolean cambiarEstado(String id, EstadoVacaciones esperado, EstadoVacaciones nuevo) {
            alCambiarEstado.run();
            if (fallarCambioDe.contains(id)) {
                throw new IllegalStateException("BD caída para " + id);
            }
            Vacaciones actual = datos.get(id);
            if (actual == null || actual.estado() != esperado) {
                return false;
            }
            datos.put(id, actual.conEstado(nuevo));
            return true;
        }

        /** Atajo de las pruebas: inserta un período tal cual, sin validaciones. */
        Vacaciones agregar(String id, String empleadoId, LocalDateTime inicio, LocalDateTime fin, EstadoVacaciones estado) {
            Vacaciones v = new Vacaciones(id, empleadoId, inicio, fin, estado, Instant.parse("2026-09-28T15:00:00Z"));
            datos.put(id, v);
            return v;
        }
    }

    static final class EmpleadosMem implements RepositorioEmpleados {
        final Map<String, EmpleadoReplica> datos = new LinkedHashMap<>();

        @Override
        public Optional<EmpleadoReplica> buscarParaProgramar(String empleadoId) {
            return Optional.ofNullable(datos.get(empleadoId));
        }

        @Override
        public Optional<EmpleadoReplica> buscar(String empleadoId) {
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
        /** Si se asigna, registra en cada publicación si había una transacción abierta. */
        TransaccionesRastreadas transacciones;
        final List<Boolean> dentroDeTransaccionAlPublicar = new ArrayList<>();
        boolean exito = true;
        /** Viola el contrato (lanza) en la publicación número n (1 = la primera); 0 = nunca. */
        int lanzarEnLaPublicacion;

        @Override
        public boolean publicar(String tipo, Object data) {
            if (transacciones != null) {
                dentroDeTransaccionAlPublicar.add(transacciones.dentro);
            }
            publicados.add(new Publicado(tipo, data));
            if (publicados.size() == lanzarEnLaPublicacion) {
                throw new IllegalStateException("fallo inesperado del publicador");
            }
            return exito;
        }
    }

    /** Como SinTransaccion, pero sabe si hay una transacción abierta y cuántas se abrieron. */
    static final class TransaccionesRastreadas implements Transacciones {
        boolean dentro;
        int abiertas;

        @Override
        public <T> T ejecutar(Supplier<T> bloque) {
            abiertas++;
            dentro = true;
            try {
                return bloque.get();
            } finally {
                dentro = false;
            }
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
