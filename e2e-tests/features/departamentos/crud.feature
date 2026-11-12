# language: es

Característica: Administración de departamentos
  Como responsable de Recursos Humanos
  Quiero registrar, actualizar y eliminar departamentos
  Para mantener al día la estructura de la empresa, sin que un empleado pueda alterarla

  Escenario: ADMIN registra, actualiza y elimina un departamento
    Dado que estoy autenticado como "ADMIN"
    Cuando registro un departamento nuevo
    Entonces la respuesta debe tener código 201
    Cuando cambio el nombre del departamento a "Finanzas E2E"
    Entonces la respuesta debe tener código 200
    Y el departamento debe tener el nombre "Finanzas E2E"
    Cuando elimino el departamento
    Entonces la respuesta debe tener código 204
    Cuando consulto el departamento
    Entonces la respuesta debe tener código 404
    Y el mensaje de error debe ser el de un departamento que no existe

  Escenario: Actualizar un departamento que no existe responde 404
    Dado que estoy autenticado como "ADMIN"
    Cuando cambio el nombre de un departamento que no existe a "Nada"
    Entonces la respuesta debe tener código 404

  Esquema del escenario: Un empleado con rol USER no puede <operación>
    Dado que existe un empleado activo con credenciales establecidas
    Y que estoy autenticado como "USER"
    Cuando solicito <operación>
    Entonces la respuesta debe tener código 403
    Y la respuesta no debe revelar detalles internos

    Ejemplos:
      | operación                  |
      | actualizar un departamento |
      | eliminar un departamento   |
