# language: es
Característica: Reprogramación de vacaciones
  Como Recursos Humanos
  Quiero cambiar las fechas de un período que todavía no empieza
  Para que el empleado tome sus vacaciones cuando se acordó, sin cancelar y volver a programar

  Reprogramar aplica las mismas reglas que programar y vuelve a publicar
  vacaciones.programadas, así el empleado recibe la confirmación con las fechas nuevas.
  Solo un período PROGRAMADA que aún no inicia se puede reprogramar.

  Antecedentes:
    Dado que existe un empleado activo con credenciales establecidas
    Y que estoy autenticado como "ADMIN"
    Y que el empleado ya tiene un período de vacaciones programado

  Escenario: Reprogramación de un período programado con confirmación al empleado
    Cuando reprogramo ese período para dentro de 90 días
    Entonces la respuesta debe tener código 200
    Y el período debe tener las fechas nuevas
    Y eventualmente el empleado debe recibir la confirmación de sus vacaciones

  Escenario: No se reprograma un período cancelado
    Dado que ese período fue cancelado
    Cuando reprogramo ese período para dentro de 90 días
    Entonces la respuesta debe tener código 409
    Y el período debe quedar en estado "CANCELADA"
