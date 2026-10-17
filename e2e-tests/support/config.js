'use strict';

// Toda la configuración sale de variables de entorno (P2): la misma suite corre
// contra el Compose local, un contenedor o Jenkins cambiando solo BASE_URL.
// Los valores por defecto son los académicos del docker-compose.yml.

function urlBase(valor, nombre) {
  let url;
  try {
    url = new URL(valor);
  } catch {
    throw new Error(`${nombre} no es una URL válida: "${valor}" (ejemplo: http://localhost:8080)`);
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    throw new Error(`${nombre} debe usar http o https: "${valor}"`);
  }
  // Sin la barra final, para concatenar rutas que empiezan con "/".
  return url.href.replace(/\/+$/, '');
}

function enteroPositivo(nombre, valor, porDefecto) {
  if (valor === undefined || valor === '') return porDefecto;
  const numero = Number(valor);
  if (!Number.isInteger(numero) || numero <= 0) {
    throw new Error(`${nombre} debe ser un entero positivo: "${valor}"`);
  }
  return numero;
}

function leerConfiguracion(entorno = process.env) {
  return Object.freeze({
    baseUrl: urlBase(entorno.BASE_URL || 'http://localhost:8080', 'BASE_URL'),
    // El token de activación completo solo viaja en el correo: GET /notificaciones lo
    // devuelve enmascarado (****1a2b) por diseño del Reto 5. Mailhog lo expone por HTTP.
    mailhogUrl: urlBase(entorno.MAILHOG_URL || 'http://localhost:8025', 'MAILHOG_URL'),
    admin: Object.freeze({
      email: entorno.ADMIN_EMAIL || 'admin@empresa.com',
      password: entorno.ADMIN_PASSWORD || 'Admin123*',
    }),
    polling: Object.freeze({
      intentos: enteroPositivo('E2E_POLL_INTENTOS', entorno.E2E_POLL_INTENTOS, 15),
      intervaloMs: enteroPositivo('E2E_POLL_INTERVALO_MS', entorno.E2E_POLL_INTERVALO_MS, 1000),
    }),
  });
}

// Se valida una sola vez al cargar la suite: una BASE_URL mal escrita detiene
// la ejecución antes del primer escenario, con un mensaje claro.
const configuracion = leerConfiguracion();

module.exports = { configuracion, leerConfiguracion };
