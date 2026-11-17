const { describe, it, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const jwt = require('jsonwebtoken');
const { crearApp } = require('../app');
const { crearHandlerEmpleadoCompleto } = require('../composicion');

const JWT_SECRET = 'secreto-pruebas-v4';
const JWT_ISSUER = 'talentflow-auth';

function firmarToken(sub = 'E001', role = 'USER', email = 'user@talentflow.local') {
  return jwt.sign({ sub, role, email }, JWT_SECRET, {
    algorithm: 'HS256',
    issuer: JWT_ISSUER,
    expiresIn: '1h'
  });
}

function escuchar(servidor) {
  return new Promise((resolve) => {
    servidor.listen(0, '127.0.0.1', () => {
      const { port } = servidor.address();
      resolve(`http://127.0.0.1:${port}`);
    });
  });
}

function cerrar(servidor) {
  return new Promise((resolve) => servidor.close(() => resolve()));
}

describe('GET /empleados/:id/completo (composición)', () => {
  const servidores = [];

  afterEach(async () => {
    while (servidores.length > 0) {
      await cerrar(servidores.pop());
    }
  });

  it('caso 1: combina empleado y perfil en paralelo, reenvía Authorization e identidad (200, parcial: false)', async () => {
    const cabecerasRecibidas = {};
    const tiemposInicio = {};

    const srvEmpleados = http.createServer((req, res) => {
      tiemposInicio.empleados = Date.now();
      cabecerasRecibidas.empleados = req.headers;
      setTimeout(() => {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ id: 'E001', nombre: 'Valeria', rol: 'USER' }));
      }, 60);
    });

    const srvPerfiles = http.createServer((req, res) => {
      tiemposInicio.perfiles = Date.now();
      cabecerasRecibidas.perfiles = req.headers;
      setTimeout(() => {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ empleadoId: 'E001', biografia: 'Ingeniera de plataforma' }));
      }, 60);
    });

    const urlEmpleados = await escuchar(srvEmpleados);
    const urlPerfiles = await escuchar(srvPerfiles);
    servidores.push(srvEmpleados, srvPerfiles);

    const app = crearApp({
      jwtSecret: JWT_SECRET,
      jwtIssuer: JWT_ISSUER,
      servicios: [
        { nombre: 'empleados', prefijo: '/empleados', url: urlEmpleados },
        { nombre: 'perfiles', prefijo: '/perfiles', url: urlPerfiles }
      ]
    });
    const srvGateway = http.createServer(app);
    const urlGateway = await escuchar(srvGateway);
    servidores.push(srvGateway);

    const token = firmarToken('E002', 'USER', 'consulta@talentflow.local');
    const inicioTotal = Date.now();
    const respuesta = await fetch(`${urlGateway}/empleados/E001/completo`, {
      headers: { Authorization: `Bearer ${token}` }
    });
    const duracionMs = Date.now() - inicioTotal;
    const cuerpo = await respuesta.json();

    assert.equal(respuesta.status, 200);
    assert.deepEqual(cuerpo, {
      empleado: { id: 'E001', nombre: 'Valeria', rol: 'USER' },
      perfil: { empleadoId: 'E001', biografia: 'Ingeniera de plataforma' },
      parcial: false,
      faltantes: []
    });

    assert.ok(Math.abs(tiemposInicio.empleados - tiemposInicio.perfiles) < 40);
    assert.ok(duracionMs < 115, `esperado paralelo (<115ms), tardó ${duracionMs}ms`);
    assert.equal(cabecerasRecibidas.empleados.authorization, `Bearer ${token}`);
    assert.equal(cabecerasRecibidas.perfiles.authorization, `Bearer ${token}`);
    assert.equal(cabecerasRecibidas.empleados['x-user-id'], 'E002');
    assert.equal(cabecerasRecibidas.perfiles['x-user-role'], 'USER');
  });

  it('caso 2: si empleados responde 404, devuelve 404 sin exponer datos parciales', async () => {
    const srvEmpleados = http.createServer((_req, res) => {
      res.writeHead(404, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ message: 'No existe' }));
    });
    const srvPerfiles = http.createServer((_req, res) => {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ empleadoId: 'E999' }));
    });

    const urlEmpleados = await escuchar(srvEmpleados);
    const urlPerfiles = await escuchar(srvPerfiles);
    servidores.push(srvEmpleados, srvPerfiles);

    const app = crearApp({
      jwtSecret: JWT_SECRET,
      jwtIssuer: JWT_ISSUER,
      servicios: [
        { nombre: 'empleados', prefijo: '/empleados', url: urlEmpleados },
        { nombre: 'perfiles', prefijo: '/perfiles', url: urlPerfiles }
      ]
    });
    const srvGateway = http.createServer(app);
    const urlGateway = await escuchar(srvGateway);
    servidores.push(srvGateway);

    const respuesta = await fetch(`${urlGateway}/empleados/E999/completo`, {
      headers: { Authorization: `Bearer ${firmarToken('admin', 'ADMIN')}` }
    });
    const cuerpo = await respuesta.json();

    assert.equal(respuesta.status, 404);
    assert.equal(cuerpo.error, 'Not Found');
    assert.equal(cuerpo.path, '/empleados/E999/completo');
  });

  it('caso 3: si empleados falla con 500 o está caído, devuelve 503', async () => {
    const srvEmpleadosError = http.createServer((_req, res) => {
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'Boom' }));
    });
    const srvPerfiles = http.createServer((_req, res) => {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ empleadoId: 'E001' }));
    });

    const urlEmpleadosError = await escuchar(srvEmpleadosError);
    const urlPerfiles = await escuchar(srvPerfiles);
    servidores.push(srvEmpleadosError, srvPerfiles);

    const appError = crearApp({
      jwtSecret: JWT_SECRET,
      jwtIssuer: JWT_ISSUER,
      servicios: [
        { nombre: 'empleados', prefijo: '/empleados', url: urlEmpleadosError },
        { nombre: 'perfiles', prefijo: '/perfiles', url: urlPerfiles }
      ]
    });
    const srvGateway1 = http.createServer(appError);
    const urlGateway1 = await escuchar(srvGateway1);
    servidores.push(srvGateway1);

    const r1 = await fetch(`${urlGateway1}/empleados/E001/completo`, {
      headers: { Authorization: `Bearer ${firmarToken()}` }
    });
    assert.equal(r1.status, 503);

    const appCaido = crearApp({
      jwtSecret: JWT_SECRET,
      jwtIssuer: JWT_ISSUER,
      servicios: [
        { nombre: 'empleados', prefijo: '/empleados', url: 'http://127.0.0.1:1' },
        { nombre: 'perfiles', prefijo: '/perfiles', url: urlPerfiles }
      ]
    });
    const srvGateway2 = http.createServer(appCaido);
    const urlGateway2 = await escuchar(srvGateway2);
    servidores.push(srvGateway2);

    const r2 = await fetch(`${urlGateway2}/empleados/E001/completo`, {
      headers: { Authorization: `Bearer ${firmarToken()}` }
    });
    assert.equal(r2.status, 503);
  });

  it('caso 4a: si perfiles responde error HTTP (500) o está caído, devuelve 200 parcial con perfil: null', async () => {
    const srvEmpleados = http.createServer((_req, res) => {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ id: 'E001', nombre: 'Valeria' }));
    });
    const srvPerfilesError = http.createServer((_req, res) => {
      res.writeHead(500);
      res.end();
    });

    const urlEmpleados = await escuchar(srvEmpleados);
    const urlPerfilesError = await escuchar(srvPerfilesError);
    servidores.push(srvEmpleados, srvPerfilesError);

    const app = crearApp({
      jwtSecret: JWT_SECRET,
      jwtIssuer: JWT_ISSUER,
      servicios: [
        { nombre: 'empleados', prefijo: '/empleados', url: urlEmpleados },
        { nombre: 'perfiles', prefijo: '/perfiles', url: urlPerfilesError }
      ]
    });
    const srvGateway = http.createServer(app);
    const urlGateway = await escuchar(srvGateway);
    servidores.push(srvGateway);

    const respuesta = await fetch(`${urlGateway}/empleados/E001/completo`, {
      headers: { Authorization: `Bearer ${firmarToken()}` }
    });
    const cuerpo = await respuesta.json();

    assert.equal(respuesta.status, 200);
    assert.deepEqual(cuerpo, {
      empleado: { id: 'E001', nombre: 'Valeria' },
      perfil: null,
      parcial: true,
      faltantes: ['perfil']
    });
  });

  it('caso 4b: si perfiles tarda más que el timeout (2 s), corta perfiles y responde 200 parcial', async () => {
    const srvEmpleados = http.createServer((_req, res) => {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ id: 'E001', nombre: 'Valeria' }));
    });
    const srvPerfilesLento = http.createServer((_req, res) => {
      const t = setTimeout(() => {
        if (!res.writableEnded) {
          res.writeHead(200, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ empleadoId: 'E001' }));
        }
      }, 200);
      t.unref();
    });

    const urlEmpleados = await escuchar(srvEmpleados);
    const urlPerfilesLento = await escuchar(srvPerfilesLento);
    servidores.push(srvEmpleados, srvPerfilesLento);

    const handler = crearHandlerEmpleadoCompleto({
      urlEmpleados,
      urlPerfiles: urlPerfilesLento,
      timeoutPerfilMs: 40
    });

    let codigoRespuesta;
    let jsonRespuesta;
    const req = {
      ruta: { segmentos: ['empleados', 'E001', 'completo'] },
      headers: {},
      originalUrl: '/empleados/E001/completo'
    };
    const res = {
      status(codigo) {
        codigoRespuesta = codigo;
        return this;
      },
      json(cuerpo) {
        jsonRespuesta = cuerpo;
        return this;
      }
    };

    await handler(req, res, (err) => {
      throw err;
    });

    assert.equal(codigoRespuesta, 200);
    assert.deepEqual(jsonRespuesta, {
      empleado: { id: 'E001', nombre: 'Valeria' },
      perfil: null,
      parcial: true,
      faltantes: ['perfil']
    });
  });

  it('cubre fallo de red en perfiles (catch de consultarPerfil) y error interno (next)', async () => {
    const handler = crearHandlerEmpleadoCompleto({
      urlEmpleados: 'http://empleados.local',
      urlPerfiles: 'http://perfiles.local',
      fetchImpl: (url) => {
        if (url.startsWith('http://empleados.local')) {
          return Promise.resolve({
            ok: true,
            status: 200,
            json: async () => ({ id: 'E001', nombre: 'Valeria' })
          });
        }
        return Promise.reject(new Error('Fallo de red en perfiles'));
      }
    });

    let codigo;
    let cuerpo;
    await handler(
      { ruta: { segmentos: ['empleados', 'E001', 'completo'] }, headers: {} },
      {
        status(c) {
          codigo = c;
          return this;
        },
        json(j) {
          cuerpo = j;
          return this;
        }
      },
      (err) => {
        throw err;
      }
    );
    assert.equal(codigo, 200);
    assert.equal(cuerpo.parcial, true);

    let errorCapturado = null;
    await handler({}, {}, (err) => {
      errorCapturado = err;
    });
    assert.ok(errorCapturado instanceof TypeError);
  });
});
