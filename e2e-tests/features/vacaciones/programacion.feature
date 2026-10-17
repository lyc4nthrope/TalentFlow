# language: es
Característica: Programación de vacaciones
  Como Recursos Humanos
  Quiero programar los períodos de vacaciones de un empleado
  Para que el sistema valide las fechas y le confirme al empleado lo que quedó programado

  Las fechas llevan hora y se calculan en la zona del servicio (America/Bogota). Los
  períodos de estos escenarios caen dentro de varias semanas, así que el scheduler no
  los toca durante la prueba. La confirmación llega por un evento
  (vacaciones.programadas): el paso con "eventualmente" la espera.

  Antecedentes:
    Dado que existe un empleado activo con credenciales establecidas
    Y que estoy autenticado como "ADMIN"

  Escenario: Programación exitosa de un período con confirmación al empleado
    Cuando programo un período de vacaciones para el empleado dentro de 60 días
    Entonces la respuesta debe tener código 201
    Y el período debe quedar en estado "PROGRAMADA"
    Y eventualmente el empleado debe recibir la confirmación de sus vacaciones

  Esquema del escenario: Rechazo de un período con fechas incoherentes: <caso>
    Cuando programo un período de vacaciones en el que <caso>
    Entonces la respuesta debe tener código 400
    Y el error debe señalar el campo "<campo>"
    Y el empleado no debe tener períodos de vacaciones registrados

    Ejemplos:
      | caso                                       | campo       |
      | la fecha de fin es anterior a la de inicio | fechaFin    |
      | la fecha de fin es igual a la de inicio    | fechaFin    |
      | la fecha de inicio ya pasó                 | fechaInicio |

  Escenario: Rechazo de un período que se cruza con otro ya programado
    Dado que el empleado ya tiene un período de vacaciones programado
    Cuando programo un período de vacaciones que se cruza con el ya programado
    Entonces la respuesta debe tener código 400
    Y el error debe señalar el campo "fechaInicio"
    Y la respuesta debe identificar el período en conflicto
    Y el empleado debe tener un solo período de vacaciones registrado
