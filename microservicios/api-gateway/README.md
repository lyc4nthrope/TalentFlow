# API Gateway (Node.js + Express)

Único punto de entrada del sistema (`http://localhost:8080`). Enruta cada prefijo a su
servicio, agrega el `/health` de todo el ecosistema y, desde el Reto 5, **valida el JWT y
decide los permisos en un solo lugar** (opción 1 del PDF).

| Archivo | Responsabilidad |
|---|---|
| `index.js` | Lee el entorno (`PORT`, `JWT_SECRET`, `JWT_ISSUER`, URL de cada servicio) y abre el puerto. `JWT_SECRET` es **obligatoria**: sin ella el Gateway escribe el motivo y termina con código 1, sin abrir el puerto (no hay secreto por defecto en el código; Compose lo inyecta desde el bloque `x-jwt`, el mismo que recibe `auth-service`). `JWT_ISSUER` vale `auth-service` por defecto |
| `app.js` | `crearApp(config)`: arma el Gateway sin red (así se prueba) |
| `autenticacion.js` | Valida el token de acceso: solo HS256, emisor `auth-service`, `exp` obligatorio, `sub` y `role` válidos |
| `autorizacion.js` | Análisis seguro de la ruta, rutas públicas y matriz de permisos |
| `salud.js` | `/health` agregado (Retos 3 y 4) |

## Orden de cada petición

1. Se borran las cabeceras `X-User-Id`, `X-User-Email` y `X-User-Role` que envíe el cliente.
2. Se analiza la ruta **tal como se va a enrutar**: cada segmento se decodifica una vez y se
   rechaza con **400** si trae `%2F`/`%5C` (barras codificadas), `.` o `..` (también
   `%2E%2E`), doble codificación (`%252F`), segmentos vacíos (`//`) o codificación inválida.
   Una ruta que no es de ningún servicio responde **404**, con o sin token.
3. Ruta pública → se reenvía sin mirar el token. Si no, sin token válido → **401**; con
   token válido pero sin permiso → **403**.
4. Con permiso, el Gateway agrega `X-User-Id` (= `sub`), `X-User-Role` y `X-User-Email`.

## Rutas públicas

| Método | Ruta |
|---|---|
| GET | `/health` |
| POST | `/auth/login`, `/auth/recover-password`, `/auth/reset-password` |
| GET / HEAD | `/<servicio>/docs`, `/<servicio>/docs/<archivo>`, `/<servicio>/openapi.json` |

## Matriz de permisos (rutas protegidas)

| Rol | Permitido |
|---|---|
| `ADMIN` | Todo |
| `USER` | `GET` y `HEAD` de cualquier recurso; `POST /auth/change-password`; `PUT /perfiles/{empleadoId}` **solo si** `empleadoId` es su propio `sub` |
| Otro caso | **403** (un USER que edita el perfil de otro recibe 403, no 401) |

## Pruebas

```bash
npm test
```

Levantan servicios falsos en puertos libres y comprueban qué se reenvía y con qué
cabeceras: la matriz completa (401 / 403 / reenvío), la propiedad del perfil, las rutas
públicas, las rutas manipuladas y el reemplazo de las cabeceras de identidad.

La instalación de la imagen usa el `package-lock.json` propio del Gateway (`npm ci`): las
versiones deben coincidir con las del lock raíz del repositorio (workspaces).
