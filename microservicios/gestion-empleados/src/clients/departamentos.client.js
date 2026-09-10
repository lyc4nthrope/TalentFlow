const { AppError } = require("../errores");

function esperar(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Cliente HTTP hacia el servicio de departamentos.
 *
 * Decisión (Reto 2, punto 6): si el servicio de departamentos no responde tras agotar
 * los reintentos, el registro del empleado se RECHAZA (no se acepta "pendiente de
 * validación"). Razón: el modelo canónico no tiene un estado que represente "validación
 * pendiente" sin inventar uno fuera de alcance de este reto, y aceptar un empleado con un
 * departamentoId sin verificar podría dejar datos inconsistentes que luego nadie revisa.
 * Se responde 503 (no 400): no es un error del cliente, es una dependencia caída.
 */
function crearClienteDepartamentos({
  baseUrl,
  timeoutMs = 2000,
  maxReintentos = 3,
  fetchImpl = fetch
} = {}) {
  async function intentarUnaVez(departamentoId) {
    const controlador = new AbortController();
    const timer = setTimeout(() => controlador.abort(), timeoutMs);
    try {
      const respuesta = await fetchImpl(`${baseUrl}/departamentos/${departamentoId}`, {
        signal: controlador.signal
      });
      if (respuesta.status === 404) {
        return false;
      }
      if (!respuesta.ok) {
        throw new Error(`Respuesta inesperada del servicio de departamentos: ${respuesta.status}`);
      }
      return true;
    } finally {
      clearTimeout(timer);
    }
  }

  async function existe(departamentoId) {
    let ultimoError;
    for (let intento = 0; intento <= maxReintentos; intento += 1) {
      try {
        return await intentarUnaVez(departamentoId);
      } catch (error) {
        ultimoError = error;
        if (intento < maxReintentos) {
          const esperaMs = 2 ** intento * 1000; // 1s, 2s, 4s, ...
          await esperar(esperaMs);
        }
      }
    }
    throw new AppError(
      `No fue posible verificar el departamento ${departamentoId}: el servicio de departamentos no respondió`,
      503
    );
    // (ultimoError queda disponible para logging si se desea en el futuro)
  }

  return { existe };
}

module.exports = { crearClienteDepartamentos };