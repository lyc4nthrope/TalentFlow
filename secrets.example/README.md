# secrets.example/

Plantillas de los archivos de secretos que se generan localmente con `scripts/setup-secrets.sh`.

**Regla del proyecto**: ningún secreto real se versiona. El repositorio debe poder publicarse como
código abierto sin comprometer credenciales (ver Restricciones del proyecto final).

- `.env` → se crea a partir de `.env.example`
- Los secretos de servicios (broker, bases de datos, observabilidad) se agregarán en los retos
  donde esos componentes entren en juego (Reto 3+, 8+).

Los archivos reales viven en `secrets/` (excluido por `.gitignore`).
