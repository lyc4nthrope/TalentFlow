# language: es
Característica: Registro de un empleado nuevo
  Como área de Recursos Humanos
  Quiero registrar a una persona que ingresa a la empresa
  Para que el sistema le prepare sus credenciales, su perfil y su aviso de bienvenida sin trabajo manual

  El alta responde en el momento, pero sus efectos llegan por eventos a otros
  servicios: perfiles consume empleado.creado; autenticación también, crea la cuenta
  y publica usuario.creado, que notificaciones convierte en el aviso de bienvenida y
  el correo. Los pasos que dicen "eventualmente" consultan varias veces hasta ver el
  efecto (como máximo 15 intentos, uno por segundo).

  El servicio de autenticación no tiene una consulta de cuentas. La evidencia de que
  creó las credenciales es el evento usuario.creado, que solo publica después de
  guardar la cuenta pendiente de activación y que llega como una notificación de
  seguridad con el token que generó. Que ese token realmente activa la cuenta se
  comprueba en activacion.feature.

  Antecedentes:
    Dado que estoy autenticado como "ADMIN"
    Y que existe un departamento para el empleado nuevo

  Escenario: El registro crea las credenciales del empleado, pendientes de activación
    Cuando registro un empleado nuevo en ese departamento
    Entonces la respuesta debe tener código 201
    Y eventualmente el servicio de autenticación debe haber creado su cuenta pendiente de activación

  Escenario: El registro envía al empleado la bienvenida con su token de activación
    Cuando registro un empleado nuevo en ese departamento
    Entonces eventualmente debe existir una notificación de bienvenida para su email
    Y la notificación no debe mostrar el token completo
    Y eventualmente debe llegar a su correo el token de activación

  Escenario: El registro crea el perfil del empleado con sus datos básicos
    Cuando registro un empleado nuevo en ese departamento
    Entonces eventualmente debe existir su perfil con su nombre completo y su email

  Escenario: No se registra un empleado en un departamento que no existe
    Cuando registro un empleado nuevo en un departamento que no existe
    Entonces la respuesta debe tener código 400
    Y el error debe señalar el campo "departamentoId"
    Y el empleado no debe haber quedado registrado

  Esquema del escenario: No se registra un empleado sin el campo obligatorio "<campo>"
    Cuando registro un empleado nuevo sin el campo "<campo>"
    Entonces la respuesta debe tener código 400
    Y el error debe señalar el campo "<campo>"

    Ejemplos:
      | campo          |
      | id             |
      | nombre         |
      | apellido       |
      | email          |
      | numeroEmpleado |
      | cargo          |
      | area           |
      | departamentoId |
      | fechaIngreso   |

  Esquema del escenario: No se registran dos empleados con el mismo <campo>
    Dado que existe otro empleado registrado
    Cuando registro un empleado nuevo con el mismo "<campo>" que el otro empleado
    Entonces la respuesta debe tener código 400
    Y el error debe señalar el campo "<campo>"
    Y el empleado no debe haber quedado registrado

    Ejemplos:
      | campo          |
      | email          |
      | numeroEmpleado |
