const TIMEOUT_PERFIL_MS = 2000;

function cabecerasReenviadas(req) {
  const cabeceras = { Accept: 'application/json' };
  for (const nombre of ['authorization', 'x-user-id', 'x-user-role', 'x-user-email']) {
    if (req.headers[nombre]) cabeceras[nombre] = req.headers[nombre];
  }
  return cabeceras;
}

async function consultarEmpleado(urlBase, id, cabeceras, fetchImpl) {
  try {
    const res = await fetchImpl(`${urlBase}/empleados/${encodeURIComponent(id)}`, {
      headers: cabeceras
    });
    if (res.status === 404) return { estado: 'no_encontrado' };
    if (!res.ok) return { estado: 'error' };
    return { estado: 'ok', datos: await res.json() };
  } catch {
    return { estado: 'error' };
  }
}

async function consultarPerfil(urlBase, id, cabeceras, timeoutMs, fetchImpl) {
  try {
    const res = await fetchImpl(`${urlBase}/perfiles/${encodeURIComponent(id)}`, {
      headers: cabeceras,
      signal: AbortSignal.timeout(timeoutMs)
    });
    if (!res.ok) return null;
    return await res.json();
  } catch {
    return null;
  }
}

/**
 * Construye el handler Express para GET /empleados/:id/completo.
 * Consulta empleados-service y perfiles-service en paralelo (2 spans hermanos en Jaeger).
 */
function crearHandlerEmpleadoCompleto({
  urlEmpleados,
  urlPerfiles,
  timeoutPerfilMs = TIMEOUT_PERFIL_MS,
  fetchImpl = fetch
}) {
  return async (req, res, next) => {
    try {
      const id = req.ruta.segmentos[1];
      const cabeceras = cabecerasReenviadas(req);

      const [resultadoEmpleado, perfil] = await Promise.all([
        consultarEmpleado(urlEmpleados, id, cabeceras, fetchImpl),
        consultarPerfil(urlPerfiles, id, cabeceras, timeoutPerfilMs, fetchImpl)
      ]);

      if (resultadoEmpleado.estado === 'no_encontrado') {
        return res.status(404).json({
          error: 'Not Found',
          message: 'Empleado no encontrado',
          path: req.originalUrl
        });
      }

      if (resultadoEmpleado.estado === 'error') {
        return res.status(503).json({
          error: 'Service Unavailable',
          message: 'El servicio de empleados no está disponible temporalmente',
          path: req.originalUrl
        });
      }

      const parcial = perfil === null;
      return res.status(200).json({
        empleado: resultadoEmpleado.datos,
        perfil,
        parcial,
        faltantes: parcial ? ['perfil'] : []
      });
    } catch (error) {
      next(error);
    }
  };
}

module.exports = { crearHandlerEmpleadoCompleto, TIMEOUT_PERFIL_MS };