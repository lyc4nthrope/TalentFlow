# language: es
Característica: Suspensión de cuenta durante vacaciones
  Como empresa
  Quiero que la cuenta del empleado se suspenda mientras está de vacaciones
  Para que nadie entre con ella durante el período y el empleado la recupere al volver

  Cómo se controla el tiempo: el período empieza unos segundos después de ahora y
  termina poco después, con fechas con hora en la zona del servicio (America/Bogota).
  El scheduler de vacaciones-service lo inicia y lo finaliza en sus pasadas; para la
  suite corre cada 5 s (docker-compose.e2e.yml), sin endpoints de disparo manual. Los
  pasos no esperan un tiempo fijo: consultan el estado del período y el login hasta
  ver la condición. La frecuencia del scheduler solo fija el tope de esa espera
  (E2E_SCHEDULER_SEGUNDOS: 5 con el override, 60 con el cron por defecto).

  "No puede iniciar sesión" se verifica sobre una precondición: el login funciona
  antes de programar y el período tiene que estar EN_CURSO. Si se agota la espera, el
  fallo dice si el período nunca empezó (scheduler) o si empezó y la cuenta sigue
  activa (auth); lo mismo al reactivarse.

  Antecedentes:
    Dado que existe un empleado activo con credenciales establecidas
    Y que estoy autenticado como "ADMIN"

  Escenario: La cuenta se suspende al iniciar las vacaciones y se reactiva al terminar
    Cuando programo un período de vacaciones que inicia de inmediato
    Entonces eventualmente el empleado no puede iniciar sesión
    Y eventualmente el empleado debe recibir el aviso "Su cuenta fue desactivada temporalmente por vacaciones."
    Cuando el período de vacaciones finaliza
    Entonces eventualmente el empleado puede iniciar sesión nuevamente
    Y eventualmente el empleado debe recibir el aviso "Bienvenido de regreso. Su cuenta fue reactivada."
