const CircuitBreaker = require("opossum");

function esperar(ms){
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
    try{
      const respuesta = await fetchImpl(`${baseUrl}/departamentos/${departamentoId}`,{
        signal: controlador.signal
      });

      if(respuesta.status === 404){
        return false;
      }
      if(!respuesta.ok){
        throw new Error(`Respuesta inesperada del servicio departamnetos`);
      }
      return true;
    }finally{
      clearTimeout(timer);
    }
  }

  async function realizarPeticionConReintentos(departamentoId) {
    let ultimoError;
    for(let intento = 0; intento <= maxReintentos; intento += 1){
      try{
        return await intentarUnaVez(departamentoId);
      } catch(error){
        ultimoError = error;
        if(intento < maxReintentos){
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

  // Marcador interno: distingue "no se pudo verificar" (dependencia caída) de
  // "se verificó y no existe" (respuesta real del servicio). El fallback ya NO
  // lanza: lanzar aquí forzaba a rechazar el registro (503) cuando departamentos
  // está caído. Por decisión del profesor, ese caso debe quedar PENDIENTE, no
  // rechazado — ver reconciliarPendientes() en empleados.service.js.
  const DEPENDENCIA_NO_DISPONIBLE = Symbol("dependencia-no-disponible");
  breaker.fallback(() => DEPENDENCIA_NO_DISPONIBLE);

  breaker.on("open", () => console.warn("⚠️ Circuit Breaker ABIERTO para Departamentos"));
  breaker.on("halfOpen", () => console.info("🔄 Circuit Breaker HALF-OPEN para Departamentos"));
  breaker.on("close", () => console.info("✅ Circuit Breaker CERRADO para Departamentos"));

  // Contrato: resuelve siempre a uno de tres valores, nunca lanza por sí mismo.
  //   'EXISTE'     -> se consultó de verdad y el departamento existe
  //   'NO_EXISTE'  -> se consultó de verdad y el departamento NO existe (404 real)
  //   'PENDIENTE'  -> no se pudo consultar (circuito abierto o dependencia caída)
  async function existe(departamentoId) {
    const resultado = await breaker.fire(departamentoId);
    if (resultado === DEPENDENCIA_NO_DISPONIBLE) return "PENDIENTE";
    return resultado ? "EXISTE" : "NO_EXISTE";
  }

  // Estado observable del circuito, para el endpoint de diagnóstico.
  function estadoActual() {
    if (breaker.opened) return "OPEN";
    if (breaker.halfOpen) return "HALF_OPEN";
    return "CLOSED";
  }

  // Se dispara cuando el circuito vuelve a CERRAR tras haber estado abierto:
  // es la señal de "el servicio se restableció" que dispara la reconciliación.
  function onRecuperado(callback) {
    breaker.on("close", callback);
  }

  return { existe, estadoActual, onRecuperado };
}

module.exports = { crearClienteDepartamentos };
