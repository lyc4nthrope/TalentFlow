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
    // JWT del ADMIN para preparar datos; ver tokenDeAdmin().
    this.tokenAdmin = null;
  }

  // Nunca lanza por un código ≠ 2xx: la verificación la hacen los pasos Entonces.
  // Solo lanza si no hay respuesta (sistema caído), con la URL en el mensaje.
  // `autorizacion` manda la cabecera Authorization tal cual (para probar esquemas o
  // tokens malformados); si no se da, se arma "Bearer <token>".
  // No toca `ultimaRespuesta`: sirve para preparar datos sin pisar lo que verifica
  // el escenario. Los pasos Cuando usan `peticion`.
  // `acepta` es la cabecera Accept: JSON para la API; una página (Swagger UI) se pide
  // con "*/*" como un navegador, porque vacaciones-service la publica solo como
  // text/html y con Accept JSON la ruta cae en GET /vacaciones/{id} (404 "docs").
  async enviar(metodo, ruta, { cuerpo, token = this.sesion.token, autorizacion, acepta = 'application/json' } = {}) {
    const url = `${this.configuracion.baseUrl}${ruta}`;
    const cabeceras = { Accept: acepta };
    if (autorizacion !== undefined) cabeceras.Authorization = autorizacion;
    else if (token) cabeceras.Authorization = `Bearer ${token}`;
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
    return {
      metodo,
      ruta,
      status: respuesta.status,
      headers: Object.fromEntries(respuesta.headers),
      body: interpretarCuerpo(await respuesta.text()),
    };
  }

  // Igual que `enviar`, pero deja la respuesta como la que verifican los Entonces.
  async peticion(metodo, ruta, opciones) {
    this.ultimaRespuesta = await this.enviar(metodo, ruta, opciones);
    return this.ultimaRespuesta;
  }

  // Login sin cambiar la sesión del escenario; devuelve el JWT.
  async iniciarSesion({ email, password }, rol = email) {
    const respuesta = await this.enviar('POST', '/auth/login', {
      cuerpo: { email, password },
      token: null,
    });
    if (respuesta.status !== 200 || !respuesta.body?.token) {
      throw new Error(
        `El login de ${rol} (${email}) devolvió ${respuesta.status}: ${extracto(respuesta.body)}`
      );
    }
    return respuesta.body.token;
  }

  // Inicia sesión y deja el token como el actual del escenario: los pasos Cuando
  // siguientes lo usan sin repetir el login.
  async autenticarComo(rol) {
    const credenciales = this.credencialesDe(rol);
    const token = await this.iniciarSesion(credenciales, rol);
    this.sesion = { token, rol, email: credenciales.email };
    return this.sesion;
  }

  // Token del ADMIN para preparar datos (crear departamentos, empleados, leer
  // notificaciones). Se pide una vez por escenario y no cambia `this.sesion`.
  async tokenDeAdmin() {
    if (!this.tokenAdmin) this.tokenAdmin = await this.iniciarSesion(this.configuracion.admin, 'ADMIN');
    return this.tokenAdmin;
  }

  // USER es el empleado que el escenario activó con
  // "Dado que existe un empleado activo con credenciales establecidas".
  credencialesDe(rol) {
    if (rol === 'ADMIN') return { ...this.configuracion.admin };
    if (rol === 'USER') {
      const empleado = this.recursos.get(ALIAS_USUARIO);
      if (!empleado?.password) {
        throw new Error(
          'El escenario no preparó un empleado con credenciales: agregue ' +
            '"Dado que existe un empleado activo con credenciales establecidas" antes de autenticarse como USER'
        );
      }
      return { email: empleado.email, password: empleado.password };
    }
    throw new Error(`No hay credenciales configuradas para el rol "${rol}" (roles: ADMIN, USER)`);
  }

  // El recurso que el escenario guardó con ese nombre lógico; si no existe, el error
  // dice qué hay, para que un paso mal ordenado se entienda sin depurar.
  recurso(alias) {
    const valor = this.recursos.get(alias);
    if (!valor) {
      const hay = [...this.recursos.keys()].join(', ') || 'ninguno';
      throw new Error(`El escenario no tiene un recurso "${alias}" (recursos: ${hay})`);
    }
    return valor;
  }
}

// Nombre lógico del empleado que actúa como USER en el escenario.
const ALIAS_USUARIO = 'empleado';

// Falla con método, ruta, código esperado y obtenido, y un extracto del cuerpo.
function exigirCodigo(respuesta, esperados, queSeHacia) {
  const lista = [].concat(esperados);
  if (lista.includes(respuesta.status)) return respuesta;
  throw new Error(
    `${queSeHacia}: se esperaba código ${lista.join(' o ')} y se obtuvo ${respuesta.status} ` +
      `en ${respuesta.metodo} ${respuesta.ruta}. Cuerpo: ${extracto(respuesta.body)}`
  );
}

function extracto(cuerpo, maximo = 300) {
  const texto = typeof cuerpo === 'string' ? cuerpo : JSON.stringify(cuerpo);
  if (texto === undefined) return '(sin cuerpo)';
  return texto.length > maximo ? `${texto.slice(0, maximo)}…` : texto;
}

setWorldConstructor(MundoTalentFlow);

module.exports = { MundoTalentFlow, extracto, exigirCodigo, ALIAS_USUARIO };
