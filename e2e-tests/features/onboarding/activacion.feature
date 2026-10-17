# language: es
Característica: Activación de la cuenta de un empleado nuevo
  Como empleado recién registrado
  Quiero establecer mi contraseña con el token que recibo por correo
  Para entrar al sistema con una contraseña que solo yo conozco

  Al registrarlo, la cuenta queda pendiente de activación y el token llega por correo
  (los pasos con "eventualmente" lo esperan). Mientras no establezca su contraseña,
  el empleado no puede entrar. El token sirve una sola vez.

  Antecedentes:
    Dado que Recursos Humanos registró a un empleado nuevo
    Y eventualmente el empleado recibe por correo su token de activación

  Escenario: Antes de activar su cuenta, el empleado no puede iniciar sesión
    Cuando el empleado intenta iniciar sesión con una contraseña que cumple la política
    Entonces la respuesta debe tener código 401
    Y el mensaje de error debe ser "credenciales inválidas"

  Escenario: El empleado establece su contraseña con el token y entra al sistema
    Cuando el empleado establece su contraseña con el token recibido
    Entonces la respuesta debe tener código 200
    Cuando el empleado inicia sesión con su nueva contraseña
    Entonces la respuesta debe tener código 200
    Y el token de acceso debe tener el rol "USER" e identificar al empleado

  Escenario: El token de activación sirve una sola vez
    Dado que el empleado ya estableció su contraseña con el token recibido
    Cuando el empleado intenta establecer otra contraseña con el mismo token
    Entonces la respuesta debe tener código 401
    Y el mensaje de error debe ser "token inválido o expirado"
    Y el empleado todavía puede iniciar sesión con su primera contraseña

  Escenario: Un token inventado no activa la cuenta
    Cuando alguien intenta establecer la contraseña del empleado con un token inventado
    Entonces la respuesta debe tener código 401
    Y el mensaje de error debe ser "token inválido o expirado"

  Esquema del escenario: Una contraseña que no cumple la política se rechaza (<motivo>)
    Cuando el empleado establece la contraseña "<contraseña>" con el token recibido
    Entonces la respuesta debe tener código 400
    Y el mensaje de error debe ser "la contraseña debe tener entre 8 y 72 bytes, al menos una mayúscula, una minúscula y un dígito"
    Y el token recibido todavía sirve para activar la cuenta

    Ejemplos:
      | contraseña     | motivo                |
      | Corta1         | menos de 8 caracteres |
      | sinmayuscula1  | sin mayúsculas        |
      | SINMINUSCULA1  | sin minúsculas        |
      | SinDigitosAqui | sin dígitos           |
