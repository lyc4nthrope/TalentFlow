# language: es

Característica: Seguridad y control de acceso
  Como sistema de autenticación
  Quiero controlar el acceso a los recursos
  Para garantizar que solo los usuarios autorizados realicen operaciones

  Escenario: Acceso denegado sin token de autenticación
    Cuando consulto la lista de empleados sin token de autenticación
    Entonces la respuesta debe tener código 401

  Esquema del escenario: Acceso denegado con una credencial inválida: <credencial>
    Cuando consulto la lista de empleados presentando "<credencial>"
    Entonces la respuesta debe tener código 401
    Y el mensaje de error debe ser "<mensaje>"
    Y la respuesta no debe revelar detalles internos

    Ejemplos:
      | credencial                                        | mensaje                                      |
      | un texto que no es un JWT                         | Token inválido o expirado.                   |
      | un token con la firma alterada                    | Token inválido o expirado.                   |
      | un token sin firma                                | Token inválido o expirado.                   |
      | un token válido con un esquema distinto de Bearer | Token requerido para acceder a este recurso. |
      | una cabecera Bearer sin token                     | Token requerido para acceder a este recurso. |

  Esquema del escenario: Las rutas públicas no exigen token: <ruta>
    Cuando consulto "<ruta>" sin token de autenticación
    Entonces la respuesta debe tener código 200

    Ejemplos:
      | ruta                    |
      | /health                 |
      | /empleados/openapi.json |
      | /vacaciones/docs        |
