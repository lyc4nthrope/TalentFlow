package com.talentflow.vacaciones.infraestructura.persistencia;

import com.talentflow.vacaciones.aplicacion.RepositorioEmpleados;
import com.talentflow.vacaciones.dominio.EmpleadoReplica;
import java.util.Optional;
import org.springframework.jdbc.core.simple.JdbcClient;

public class JdbcRepositorioEmpleados implements RepositorioEmpleados {

    private final JdbcClient jdbc;

    public JdbcRepositorioEmpleados(JdbcClient jdbc) {
        this.jdbc = jdbc;
    }

    @Override
    public Optional<EmpleadoReplica> buscarParaProgramar(String empleadoId) {
        // FOR UPDATE: bloquea la fila del empleado hasta el fin de la transacción, así dos
        // solicitudes simultáneas del mismo empleado se validan una después de la otra.
        return jdbc.sql("SELECT empleado_id, email, retirado FROM empleados_replica WHERE empleado_id = :id FOR UPDATE")
                .param("id", empleadoId)
                .query((fila, n) -> new EmpleadoReplica(
                        fila.getString("empleado_id"), fila.getString("email"), fila.getBoolean("retirado")))
                .optional();
    }

    @Override
    public void registrar(String empleadoId, String email) {
        jdbc.sql("""
                INSERT INTO empleados_replica (empleado_id, email) VALUES (:id, :email)
                ON CONFLICT (empleado_id) DO NOTHING
                """)
                .param("id", empleadoId)
                .param("email", email)
                .update();
    }

    @Override
    public void actualizarEmail(String empleadoId, String email) {
        // Upsert: si el empleado.creado se perdió, este evento lo registra igual.
        jdbc.sql("""
                INSERT INTO empleados_replica (empleado_id, email) VALUES (:id, :email)
                ON CONFLICT (empleado_id) DO UPDATE SET email = EXCLUDED.email, actualizado_en = now()
                """)
                .param("id", empleadoId)
                .param("email", email)
                .update();
    }

    @Override
    public void marcarRetirado(String empleadoId, String email) {
        jdbc.sql("""
                INSERT INTO empleados_replica (empleado_id, email, retirado) VALUES (:id, :email, true)
                ON CONFLICT (empleado_id) DO UPDATE SET retirado = true, actualizado_en = now()
                """)
                .param("id", empleadoId)
                .param("email", email)
                .update();
    }
}
