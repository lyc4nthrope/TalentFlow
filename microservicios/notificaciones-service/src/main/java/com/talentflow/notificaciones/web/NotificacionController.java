package com.talentflow.notificaciones.web;

import com.talentflow.notificaciones.dominio.Notificacion;
import com.talentflow.notificaciones.repositorio.NotificacionRepository;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.Parameter;
import io.swagger.v3.oas.annotations.tags.Tag;
import java.util.List;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/notificaciones")
@Tag(name = "Notificaciones")
public class NotificacionController {

    private final NotificacionRepository repositorio;

    public NotificacionController(NotificacionRepository repositorio) {
        this.repositorio = repositorio;
    }

    @GetMapping
    @Operation(summary = "Lista todas las notificaciones registradas",
            description = "Orden cronológico (la más antigua primero).")
    public List<Notificacion> listar() {
        return repositorio.listar();
    }

    @GetMapping("/{empleadoId}")
    @Operation(summary = "Lista las notificaciones de un empleado",
            description = "Responde 200 con lista vacía si el empleado no tiene notificaciones.")
    public List<Notificacion> listarPorEmpleado(
            @Parameter(description = "Identificador del empleado", example = "E001")
            @PathVariable String empleadoId) {
        return repositorio.listarPorEmpleado(empleadoId);
    }
}
