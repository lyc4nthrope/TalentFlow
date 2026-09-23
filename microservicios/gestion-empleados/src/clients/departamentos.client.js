const CircuitBreaker = require("opossum");

function esperar(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function crearClienteDepartamentos({
  baseUrl,
  timeoutMs = 5000,
  errorThresholdPercentage = 50,
  resetTimeout = 30000,
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
        throw new Error(`Respuesta inesperada del servicio departamentos`);
      }
      return true;
    } finally {
      clearTimeout(timer);
    }
  }

  async function realizarPeticionConReintentos(departamentoId) {
    let ultimoError;
    for (let intento = 0; intento <= maxReintentos; intento += 1) {
      try {
        return await intentarUnaVez(departamentoId);
      } catch (error) {
        ultimoError = error;
        if (intento < maxReintentos) {
          await esperar(200);
        }
      }
    }
    throw ultimoError;
  }

  const breakerOptions = {
    timeout: (timeoutMs * (maxReintentos + 1)) + 5000,
    errorThresholdPercentage,
    resetTimeout,
    volumeThreshold: 4,
    capacity: 10
  };

  const breaker = new CircuitBreaker(realizarPeticionConReintentos, breakerOptions);

  // El fallback ya NO lanza un error de negocio (antes lanzaba AppError 503).
  // Devuelve null para que el llamador (servicioEmpleados) pueda distinguir
  // tres casos: true (existe), false (confirmado que no existe, 404 real),
  // null (no se pudo verificar: circuito abierto o reintentos agotados).
  // La decisión de qué hacer con "null" es del servicio de negocio, no del cliente HTTP.
  breaker.fallback(() => null);

  breaker.on("open", () => console.warn(" Circuit Breaker ABIERTO para Departamentos"));
  breaker.on("halfOpen", () => console.info("🔄 Circuit Breaker HALF-OPEN para Departamentos"));
  breaker.on("close", () => console.info(" Circuit Breaker CERRADO para Departamentos"));

  async function existe(departamentoId) {
    return await breaker.fire(departamentoId);
  }

  return { existe, breaker };
}

module.exports = { crearClienteDepartamentos };