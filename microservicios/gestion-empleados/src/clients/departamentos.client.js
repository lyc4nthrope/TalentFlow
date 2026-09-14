const CircuitBreaker = require("opossum");
const { AppError } = require("../errores");

function esperar(ms){
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function crearClienteDepartamentos({
  baseUrl,
  timeoutMs = 3000,
  errorThresholdPercentage = 50,
  resetTimeout = 10000,
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
    capacity: 10
  };

  const breaker = new CircuitBreaker(realizarPeticionConReintentos, breakerOptions);

  breaker.fallback((departamentoId, err) => {
    throw new AppError(
      `No fue posible verificar el departamento, el servicio no resolvió`,
      503
    );
  });

  breaker.on("open", () => console.warn("⚠️ Circuit Breaker ABIERTO para Departamentos"));
  breaker.on("halfOpen", () => console.info("🔄 Circuit Breaker HALF-OPEN para Departamentos"));
  breaker.on("close", () => console.info("✅ Circuit Breaker CERRADO para Departamentos"));

  // 4. Exponer el método 'existe' envolviéndolo en el circuito
  async function existe(departamentoId) {
    return await breaker.fire(departamentoId);
  }

  return { existe };
}

module.exports = { crearClienteDepartamentos };
