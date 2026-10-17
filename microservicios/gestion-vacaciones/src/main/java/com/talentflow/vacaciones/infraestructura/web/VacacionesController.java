package com.talentflow.vacaciones.infraestructura.web;

import com.talentflow.vacaciones.aplicacion.ServicioVacaciones;
import com.talentflow.vacaciones.aplicacion.SolicitudVacaciones;
import com.talentflow.vacaciones.dominio.Vacaciones;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.Parameter;
import io.swagger.v3.oas.annotations.responses.ApiResponse;
import io.swagger.v3.oas.annotations.tags.Tag;
import jakarta.validation.Valid;
import java.net.URI;
import java.util.List;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

/** REST para RR. HH.: solo traduce HTTP a casos de uso; la lógica vive en ServicioVacaciones. */
@RestController
@RequestMapping("/vacaciones")
@Tag(name = "Vacaciones", description = "Programación de períodos de vacaciones")
public class VacacionesController {

    private final ServicioVacaciones servicio;

    public VacacionesController(ServicioVacaciones servicio) {
        this.servicio = servicio;
    }

    @PostMapping
    @Operation(summary = "Programa un período de vacaciones y publica vacaciones.programadas")
    @ApiResponse(responseCode = "201", description = "Período PROGRAMADA (header Location con su URL)")
    @ApiResponse(responseCode = "400", description = "Fechas incoherentes, en el pasado, solapamiento "
            + "(incluye periodoEnConflicto) o empleado inexistente/retirado")
    public ResponseEntity<Vacaciones> programar(@Valid @RequestBody SolicitudVacaciones solicitud) {
        Vacaciones creada = servicio.programar(solicitud);
        return ResponseEntity.created(URI.create("/vacaciones/" + creada.id())).body(creada);
    }

    @GetMapping
    @Operation(summary = "Lista los períodos, o los de un empleado con ?empleadoId=")
    public List<Vacaciones> listar(
            @Parameter(description = "Filtra por empleado", example = "E001")
            @RequestParam(required = false) String empleadoId) {
        return servicio.listar(empleadoId);
    }

    @GetMapping("/{id}")
    @Operation(summary = "Consulta un período por su identificador")
    @ApiResponse(responseCode = "404", description = "El período no existe")
    public Vacaciones consultar(@PathVariable String id) {
        return servicio.consultar(id);
    }

    @DeleteMapping("/{id}")
    @Operation(summary = "Cancela un período que aún no ha iniciado (queda CANCELADA, no se borra)")
    @ApiResponse(responseCode = "404", description = "El período no existe")
    @ApiResponse(responseCode = "409", description = "El período ya inició o no está PROGRAMADA")
    public Vacaciones cancelar(@PathVariable String id) {
        return servicio.cancelar(id);
    }
}
