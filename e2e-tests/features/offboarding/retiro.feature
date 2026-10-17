# language: es
Característica: Desvinculación de un empleado
  Como área de Recursos Humanos
  Quiero retirar a un empleado que deja la empresa
  Para que pierda el acceso, reciba el aviso y su retiro quede registrado para auditoría

  El retiro es una baja lógica: el empleado no se borra, queda RETIRADO con la fecha y
  el motivo. Sus efectos llegan por eventos: el servicio de autenticación desactiva la
  cuenta para siempre y notificaciones le avisa. Los pasos con "eventualmente" los
  esperan; si se agota la espera, el fallo dice si el retiro no se aplicó en empleados o
  si empleados lo retiró y la cuenta siguió activa.

  Cómo se prueba que la recuperación de contraseña "falla": el sistema responde lo mismo
  (202) a cualquier correo, exista o no y esté retirado o no, para no revelar qué
  cuentas existen. Por eso el fallo no se ve en la respuesta, sino en lo que no pasa:
  nunca le llega un token. Para que esa ausencia signifique algo, en el mismo escenario
  el empleado pide la recuperación antes del retiro y sí le llega el token (en
  milisegundos): así se sabe que el camino funciona y cuánto tarda. Después del retiro se
  vigila el historial de notificaciones y el correo durante al menos 3 segundos, más de
  cien veces lo que tarda un evento en llegar, y nunca menos que el triple de lo que tardó
  ese primer pedido. Además, el token que le llegó antes del retiro ya no sirve.

  Antecedentes:
    Dado que existe un empleado activo con credenciales establecidas
    Y que estoy autenticado como "ADMIN"

  Escenario: Desvinculación completa con aviso al empleado
    Cuando solicito retirar a un empleado
    Entonces la respuesta debe tener código 200
    Y el empleado debe quedar en estado "RETIRADO"
    Y eventualmente el empleado debe recibir el aviso "Su cuenta fue desactivada de forma permanente por retiro."

  Escenario: El empleado retirado ya no puede iniciar sesión
    Cuando solicito retirar a un empleado
    Entonces la respuesta debe tener código 200
    Y eventualmente el empleado retirado no puede iniciar sesión

  Escenario: La recuperación de contraseña no le sirve a un empleado retirado
    Dado que el empleado pidió recuperar su contraseña y recibió el token por correo
    Y que Recursos Humanos retiró al empleado
    Y eventualmente el empleado retirado no puede iniciar sesión
    Cuando el empleado retirado solicita recuperar su contraseña
    Entonces la respuesta debe tener código 202
    Y la respuesta debe ser igual a la que recibe un correo que no está registrado
    Y no debe llegarle ningún token de recuperación nuevo
    Y el token de recuperación que recibió antes del retiro ya no sirve

  Escenario: El empleado retirado aparece en la auditoría de retiros
    Dado que existe otro empleado registrado
    Y que Recursos Humanos retiró al empleado
    Cuando consulto la auditoría de empleados retirados
    Entonces la respuesta debe tener código 200
    Y el empleado retirado debe aparecer con la fecha de hoy y el motivo "RENUNCIA"
    Y el otro empleado, que sigue activo, no debe aparecer en la auditoría

  Escenario: Un empleado retirado durante sus vacaciones no recupera la cuenta al terminar
    Cuando programo un período de vacaciones que inicia de inmediato
    Entonces eventualmente el empleado no puede iniciar sesión
    Cuando solicito retirar a un empleado
    Entonces la respuesta debe tener código 200
    Y eventualmente el empleado debe recibir el aviso "Su cuenta fue desactivada de forma permanente por retiro."
    Cuando el período de vacaciones finaliza
    Entonces la cuenta del empleado retirado no se reactiva
