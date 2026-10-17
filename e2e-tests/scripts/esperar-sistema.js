'use strict';

// Espera a que el sistema esté listo antes de la suite (Reto 6, F11): consulta
// GET {BASE_URL}/health hasta que el Gateway informe "sistema": "OK" (todos los
// servicios arriba, con su base de datos y su broker). El Gateway responde 200
// mientras él mismo esté vivo, aunque otro servicio esté caído, así que no basta
// con su healthcheck de Docker. Es polling con tope, no una espera fija: termina
// apenas el sistema está OK y, al agotarse, sale con código 1 diciendo qué falta.
//
// Uso: node scripts/esperar-sistema.js   (E2E_ESPERA_INTENTOS, E2E_ESPERA_INTERVALO_MS)

const { configuracion } = require('../support/config');

const intentos = Number(process.env.E2E_ESPERA_INTENTOS || 60);
const intervaloMs = Number(process.env.E2E_ESPERA_INTERVALO_MS || 2000);
const url = `${configuracion.baseUrl}/health`;

const pausa = (ms) => new Promise((resolver) => setTimeout(resolver, ms));

async function estado() {
  try {
    const respuesta = await fetch(url, { signal: AbortSignal.timeout(5000) });
    const cuerpo = await respuesta.json().catch(() => ({}));
    if (cuerpo.sistema === 'OK') return { listo: true };
    return { listo: false, detalle: `sistema=${cuerpo.sistema ?? '?'} problemas=${JSON.stringify(cuerpo.problemas ?? [])}` };
  } catch (error) {
    return { listo: false, detalle: `sin respuesta: ${error.cause?.code || error.message}` };
  }
}

(async () => {
  const inicio = Date.now();
  let ultimo = '';
  for (let intento = 1; intento <= intentos; intento += 1) {
    const { listo, detalle } = await estado();
    if (listo) {
      console.log(`Sistema OK en ${url} (intento ${intento}, ${Date.now() - inicio} ms).`);
      return;
    }
    ultimo = detalle;
    if (intento < intentos) await pausa(intervaloMs);
  }
  console.error(`El sistema no quedó OK en ${url} después de ${intentos} intentos (${Date.now() - inicio} ms). Último estado: ${ultimo}`);
  process.exit(1);
})();
