# ==========================================
# CONFIGURACIÓN GLOBAL Y SEGURIDAD (JWT)
# ==========================================
JWT_SECRET=tu_clave_secreta_super_segura_reto5_2026
ZONA_HORARIA=America/Bogota

# ==========================================
# BROKER DE MENSAJERÍA (RabbitMQ)
# ==========================================
RABBITMQ_USER=admin
RABBITMQ_PASS=admin_password_5672

# ==========================================
# BASES DE DATOS DE MICROSERVICIOS
# ==========================================

# 1. Autenticación (PostgreSQL - Reto 5)
DB_AUTH_NAME=db_auth
DB_AUTH_USER=user_auth
DB_AUTH_PASS=pass_auth_secret

# 2. Empleados (PostgreSQL)
DB_EMPLEADOS_NAME=db_empleados
DB_EMPLEADOS_USER=user_empleados
DB_EMPLEADOS_PASS=pass_empleados_secret

# 3. Departamentos (MySQL)
DB_DEPTO_NAME=db_deptos
DB_DEPTO_USER=user_deptos
DB_DEPTO_PASS=pass_deptos_secret
DB_DEPTO_ROOT_PASS=root_pass_secret

# 4. Notificaciones (PostgreSQL)
DB_NOTIF_NAME=db_notificaciones
DB_NOTIF_USER=user_notificaciones
DB_NOTIF_PASS=pass_notificaciones_secret

# 5. Perfiles (PostgreSQL)
DB_PERFILES_NAME=db_perfiles
DB_PERFILES_USER=user_perfiles
DB_PERFILES_PASS=pass_perfiles_secret

# 6. Vacaciones (PostgreSQL)
DB_VACACIONES_NAME=db_vacaciones
DB_VACACIONES_USER=user_vacaciones
DB_VACACIONES_PASS=pass_vacaciones_secret
