# language: es

Característica: Permisos según el rol del usuario
  Como responsable de la seguridad de TalentFlow
  Quiero que cada rol solo pueda hacer lo que le corresponde
  Para que un empleado consulte la información sin poder alterarla, y RR. HH. la administre

  Antecedentes:
    Dado que existe un empleado activo con credenciales establecidas

  Esquema del escenario: Un empleado con rol USER puede consultar <recurso>
    Dado que estoy autenticado como "USER"
    Cuando consulto la lista de "<recurso>"
    Entonces la respuesta debe tener código 200

    Ejemplos:
      | recurso        |
      | empleados      |
      | departamentos  |
      | perfiles       |
      | vacaciones     |
      | notificaciones |

  Esquema del escenario: Un empleado con rol USER no puede <operación>
    Dado que estoy autenticado como "USER"
    Cuando solicito <operación>
    Entonces la respuesta debe tener código 403
    Y la respuesta no debe revelar detalles internos

    Ejemplos:
      | operación                             |
      | registrar un empleado nuevo           |
      | actualizar los datos de un empleado   |
      | retirar a un empleado                 |
      | crear un departamento                 |
      | programar vacaciones para un empleado |

  Escenario: Un empleado con rol USER solo puede editar su propio perfil
    Dado que existe otro empleado registrado
    Y que estoy autenticado como "USER"
    Cuando actualizo la ciudad de mi perfil a "Medellín"
    Entonces la respuesta debe tener código 200
    Y mi perfil debe tener la ciudad "Medellín"
    Cuando intento actualizar la ciudad del perfil del otro empleado a "Cali"
    Entonces la respuesta debe tener código 403

  Esquema del escenario: Un usuario con rol ADMIN puede <operación>
    Dado que estoy autenticado como "ADMIN"
    Cuando solicito <operación>
    Entonces la respuesta debe tener código <código>

    Ejemplos:
      | operación                             | código |
      | registrar un empleado nuevo           | 201    |
      | actualizar los datos de un empleado   | 200    |
      | retirar a un empleado                 | 200    |
      | crear un departamento                 | 201    |
      | programar vacaciones para un empleado | 201    |
      | editar el perfil de un empleado       | 200    |
