'use strict';

const { setWorldConstructor, World } = require('@cucumber/cucumber');
const { configuracion } = require('./config');

// Cuerpo como JSON cuando se puede y como texto cuando no (un 502 del proxy o
// un error de texto plano también se tienen que poder mostrar en el fallo).
function interpretarCuerpo(texto) {
  if (texto === '') return null;
  try {
    return JSON.parse(texto);
  } catch {
    return texto;
  }
}

// Contexto compartido (D2). Cucumber crea una instancia nueva por escenario, así
// que el token, la última respuesta y los recursos nunca pasan de un escenario
// a otro, y cada uno se puede ejecutar solo o en cualquier orden.
class MundoTalentFlow extends World {
  constructor(opciones) {
    super(opciones);
    this.configuracion = configuracion;
    // El token con el que salen las peticiones siguientes y a quién pertenece.
    this.sesion = { token: null, rol: null, email: null };
    this.ultimaRespuesta = null;
    // Lo que crea el escenario (ids, emails), por nombre lógico: "empleado", etc.
    this.recursos = new Map();
  }

  // Nunca lanza por un código ≠ 2xx: la verificación la hacen los pasos Entonces.
  // Solo lanza si no hay respuesta (sistema caído), con la URL en el mensaje.
  async peticion(metodo, ruta, { cuerpo, token = this.sesion.token } = {}) {
    const url = `${this.configuracion.baseUrl}${ruta}`;
    const cabeceras = { Accept: 'application/json' };
    if (token) cabeceras.Authorization = `Bearer ${token}`;
    if (cuerpo !== undefined) cabeceras['Content-Type'] = 'application/json';

    let respuesta;
    try {
      respuesta = await fetch(url, {
        method: metodo,
        headers: cabeceras,
        body: cuerpo === undefined ? undefined : JSON.stringify(cuerpo),
        signal: AbortSignal.timeout(10_000),
      });
    } catch (error) {
      const causa = error.cause?.code || error.cause?.message || error.message;
      throw new Error(`${metodo} ${url} no obtuvo respuesta: ${causa}`);
    }
    this.ultimaRespuesta = {
      metodo,
      ruta,
      status: respuesta.status,
      headers: Object.fromEntries(respuesta.headers),
      body: interpretarCuerpo(await respuesta.text()),
    };
    return this.ultimaRespuesta;
  }

  // Inicia sesión y deja el token como el actual del escenario: los pasos Cuando
  // siguientes lo usan sin repetir el login. Para USER se agregará el caso de un
  // empleado activado por la suite (F7/F8); hoy solo existe el ADMIN semilla.
  async autenticarComo(rol) {
    const credenciales = this.credencialesDe(rol);
    const respuesta = await this.peticion('POST', '/auth/login', {
      cuerpo: credenciales,
      token: null,
    });
    if (respuesta.status !== 200 || !respuesta.body?.token) {
      throw new Error(
        `El login de ${rol} (${credenciales.email}) devolvió ${respuesta.status}: ${extracto(respuesta.body)}`
      );
    }
    this.sesion = { token: respuesta.body.token, rol, email: credenciales.email };
    return this.sesion;
  }

  credencialesDe(rol) {
    if (rol === 'ADMIN') return { ...this.configuracion.admin };
    throw new Error(`No hay credenciales configuradas para el rol "${rol}"`);
  }
}

function extracto(cuerpo, maximo = 300) {
  const texto = typeof cuerpo === 'string' ? cuerpo : JSON.stringify(cuerpo);
  if (texto === undefined) return '(sin cuerpo)';
  return texto.length > maximo ? `${texto.slice(0, maximo)}…` : texto;
}

setWorldConstructor(MundoTalentFlow);

module.exports = { MundoTalentFlow, extracto };
