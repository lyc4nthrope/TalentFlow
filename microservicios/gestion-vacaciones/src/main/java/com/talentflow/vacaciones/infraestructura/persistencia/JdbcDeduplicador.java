package com.talentflow.vacaciones.infraestructura.persistencia;

import com.talentflow.vacaciones.aplicacion.Deduplicador;
import org.springframework.jdbc.core.simple.JdbcClient;

public class JdbcDeduplicador implements Deduplicador {

    private final JdbcClient jdbc;

    public JdbcDeduplicador(JdbcClient jdbc) {
        this.jdbc = jdbc;
    }

    @Override
    public boolean registrarSiEsNuevo(String eventoId) {
        // ON CONFLICT DO NOTHING: con dos transacciones concurrentes sobre el mismo id, la
        // segunda espera a la primera y luego no inserta (0 filas): nunca hay doble efecto.
        return jdbc.sql("INSERT INTO eventos_procesados (id) VALUES (:id) ON CONFLICT (id) DO NOTHING")
                .param("id", eventoId)
                .update() == 1;
    }
}
