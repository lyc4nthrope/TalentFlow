const { describe, it, before, after } = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const jwt = require('jsonwebtoken');

const { crearApp } = require('../app');

const SECRETO = 'secreto-de-prueba';
const EMISOR = 'auth-service';
const PREFIJOS = ['/auth', '/empleados', '/departamentos', '/perfiles', '/notificaciones', '/vacaciones'];

// Servicio falso: responde 200 y recuerda la última petición que le llegó (método, url
// cruda y cabeceras), para comprobar qué reenvía el Gateway.
function servicioFalso() {
    const recibidas = [];
    const servidor = http.createServer((req, res) => {
        recibidas.push({ method: req.method, url: req.url, headers: req.headers });
        res.writeHead(200, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ ok: true }));
    });
    return { servidor, recibidas };
}

const escuchar = (servidor) =>
    new Promise((resolve) => servidor.listen(0, '127.0.0.1', () => resolve(servidor.address().port)));

// Petición HTTP cruda: la ruta viaja tal cual (fetch normalizaría "..", "%2E%2E" y otras).
function pedir(puerto, metodo, ruta, cabeceras = {}) {
    return new Promise((resolve, reject) => {
        const req = http.request({ host: '127.0.0.1', port: puerto, method: metodo, path: ruta, headers: cabeceras }, (res) => {
            let cuerpo = '';
            res.on('data', (c) => (cuerpo += c));
            res.on('error', reject);
            res.on('end', () => {
                let datos = null;
                try {
                    datos = cuerpo ? JSON.parse(cuerpo) : null;
                } catch {
                    datos = { noJson: cuerpo }; // p. ej. la página HTML de error por defecto de Express
                }
                resolve({ status: res.statusCode, cuerpo: datos });
            });
        });
        req.on('error', reject);
        req.end();
    });
}

const token = (claims, opciones = {}) =>
    jwt.sign(claims, SECRETO, { algorithm: 'HS256', issuer: EMISOR, expiresIn: '1h', ...opciones });

const ADMIN = { Authorization: `Bearer ${token({ sub: 'admin', role: 'ADMIN', email: 'admin@empresa.com' })}` };
const USER = { Authorization: `Bearer ${token({ sub: 'E001', role: 'USER', email: 'e1@empresa.com' })}` };

describe('API Gateway: autenticación, autorización y enrutamiento', () => {
    const falsos = {};
    let gateway;
    let puerto;

    // Cuántas peticiones recibió cada servicio falso antes de una llamada: así se sabe si
    // la última fue reenviada.
    const recibidasPor = (prefijo) => falsos[prefijo].recibidas;
    async function reenviada(prefijo, metodo, ruta, cabeceras) {
        const antes = recibidasPor(prefijo).length;
        const respuesta = await pedir(puerto, metodo, ruta, cabeceras);
        const nuevas = recibidasPor(prefijo).slice(antes);
        return { ...respuesta, reenvio: nuevas[0] };
    }
    const algunaReenviada = () => PREFIJOS.reduce((n, p) => n + recibidasPor(p).length, 0);

    before(async () => {
        const servicios = [];
        for (const prefijo of PREFIJOS) {
            falsos[prefijo] = servicioFalso();
            const p = await escuchar(falsos[prefijo].servidor);
            servicios.push({ nombre: prefijo.slice(1), prefijo, url: `http://127.0.0.1:${p}` });
        }
        const app = crearApp({
            jwtSecret: SECRETO,
            jwtIssuer: EMISOR,
            servicios,
            salud: async () => ({ sistema: 'OK', origen: 'salud-de-prueba' })
        });
        gateway = app.listen(0, '127.0.0.1');
        await new Promise((r) => gateway.once('listening', r));
        puerto = gateway.address().port;
    });

    after(() => {
        gateway.close();
        for (const { servidor } of Object.values(falsos)) servidor.close();
    });

    describe('401: sin token válido en una ruta protegida', () => {
        const casos = [
            ['sin token', {}],
            ['esquema no Bearer', { Authorization: `Basic ${token({ sub: 'E001', role: 'USER' })}` }],
            ['firma inválida', { Authorization: `Bearer ${jwt.sign({ sub: 'E001', role: 'USER' }, 'otra-clave', { issuer: EMISOR, expiresIn: '1h' })}` }],
            ['vencido', { Authorization: `Bearer ${token({ sub: 'E001', role: 'USER' }, { expiresIn: -10 })}` }],
            ['sin exp', { Authorization: `Bearer ${jwt.sign({ sub: 'E001', role: 'USER' }, SECRETO, { issuer: EMISOR })}` }],
            ['otro emisor', { Authorization: `Bearer ${token({ sub: 'E001', role: 'USER' }, { issuer: 'otro' })}` }],
            ['HS512 con la misma clave', { Authorization: `Bearer ${jwt.sign({ sub: 'E001', role: 'USER' }, SECRETO, { algorithm: 'HS512', issuer: EMISOR, expiresIn: '1h' })}` }],
            ['rol desconocido', { Authorization: `Bearer ${token({ sub: 'E001', role: 'ROOT' })}` }],
            ['sin sub', { Authorization: `Bearer ${token({ role: 'USER' })}` }],
            ['sub numérico', { Authorization: `Bearer ${token({ sub: 123, role: 'USER' })}` }],
            ['sub vacío', { Authorization: `Bearer ${token({ sub: '', role: 'USER' })}` }],
            ['rol como arreglo', { Authorization: `Bearer ${token({ sub: 'admin', role: ['ADMIN'] })}` }],
            ['rol vacío', { Authorization: `Bearer ${token({ sub: 'E001', role: '' })}` }],
            ['rol en minúsculas', { Authorization: `Bearer ${token({ sub: 'admin', role: 'admin' })}` }]
        ];
        for (const [nombre, cabeceras] of casos) {
            it(nombre, async () => {
                const r = await reenviada('/empleados', 'GET', '/empleados', cabeceras);
                assert.equal(r.status, 401);
                assert.equal(r.cuerpo.error, 'UNAUTHORIZED');
                assert.equal(r.reenvio, undefined, 'no debe llegar al servicio');
            });
        }
    });

    describe('ADMIN: acceso total', () => {
        for (const prefijo of PREFIJOS) {
            for (const metodo of ['POST', 'PUT', 'DELETE']) {
                it(`${metodo} ${prefijo}/X se reenvía`, async () => {
                    const r = await reenviada(prefijo, metodo, `${prefijo}/X`, ADMIN);
                    assert.equal(r.status, 200);
                    assert.equal(r.reenvio.method, metodo);
                    assert.equal(r.reenvio.url, `${prefijo}/X`);
                });
            }
        }
    });

    describe('USER: solo lectura', () => {
        for (const prefijo of PREFIJOS) {
            it(`GET ${prefijo}/X se reenvía`, async () => {
                const r = await reenviada(prefijo, 'GET', `${prefijo}/X?pagina=1`, USER);
                assert.equal(r.status, 200);
                assert.equal(r.reenvio.url, `${prefijo}/X?pagina=1`);
            });
        }

        it('HEAD sin token → 401', async () => {
            const r = await reenviada('/empleados', 'HEAD', '/empleados/E002', {});
            assert.equal(r.status, 401);
            assert.equal(r.reenvio, undefined);
        });

        it('OPTIONS no es lectura: USER → 403, sin token → 401', async () => {
            const antes = algunaReenviada();
            assert.equal((await pedir(puerto, 'OPTIONS', '/empleados', USER)).status, 403);
            assert.equal((await pedir(puerto, 'OPTIONS', '/empleados', {})).status, 401);
            assert.equal(algunaReenviada(), antes);
        });

        it('HEAD también es lectura', async () => {
            const r = await reenviada('/empleados', 'HEAD', '/empleados/E002', USER);
            assert.equal(r.status, 200);
            assert.equal(r.reenvio.method, 'HEAD');
        });

        const escrituras = [
            ['POST', '/empleados'],
            ['DELETE', '/empleados/E001'],
            ['PUT', '/empleados/E001'],
            ['POST', '/departamentos'],
            ['POST', '/vacaciones'],
            ['DELETE', '/vacaciones/V-2026-0001'],
            ['POST', '/perfiles/E001'],
            ['DELETE', '/perfiles/E001'],
            ['PATCH', '/perfiles/E001'],
            ['POST', '/notificaciones'],
            ['POST', '/auth/otra-cosa']
        ];
        for (const [metodo, ruta] of escrituras) {
            it(`${metodo} ${ruta} → 403`, async () => {
                const antes = algunaReenviada();
                const r = await pedir(puerto, metodo, ruta, USER);
                assert.equal(r.status, 403);
                assert.equal(r.cuerpo.error, 'FORBIDDEN');
                assert.equal(algunaReenviada(), antes, 'no debe llegar a ningún servicio');
            });
        }

        it('POST /auth/change-password se reenvía', async () => {
            const r = await reenviada('/auth', 'POST', '/auth/change-password', USER);
            assert.equal(r.status, 200);
            assert.equal(r.reenvio.url, '/auth/change-password');
        });
    });

    describe('USER: propiedad del perfil (PUT /perfiles/{empleadoId})', () => {
        it('su propio perfil se reenvía', async () => {
            const r = await reenviada('/perfiles', 'PUT', '/perfiles/E001', USER);
            assert.equal(r.status, 200);
            assert.equal(r.reenvio.url, '/perfiles/E001');
        });

        it('el perfil de otro empleado → 403 (no 401)', async () => {
            const r = await reenviada('/perfiles', 'PUT', '/perfiles/E002', USER);
            assert.equal(r.status, 403);
            assert.equal(r.reenvio, undefined);
        });

        it('ADMIN puede editar cualquier perfil', async () => {
            const r = await reenviada('/perfiles', 'PUT', '/perfiles/E002', ADMIN);
            assert.equal(r.status, 200);
        });
    });

    describe('Rutas manipuladas: no permiten saltarse las reglas', () => {
        const trucos = [
            '/perfiles/E002%2F..%2FE001',
            '/perfiles/E001%2F..%2FE002',
            '/perfiles/E002/%2E%2E/E001',
            '/perfiles/E002/%2e%2e/E001',
            '/perfiles/E002/../E001',
            '/perfiles/./E002',
            '/perfiles//E001',
            '/perfiles/E002%5C..%5CE001',
            '/perfiles/E002\\..\\E001',
            '/perfiles/%E0%A4%A',
            '/perfiles/E002%252F..%252FE001',
            '/perfiles/E002/%252E%252E/E001',
            '/perfiles/E001;x',
            '/perfiles/E001%3Bx',
            '/perfiles/E001%253Bx',
            '/perfiles/E002/..;/E001'
        ];
        for (const ruta of trucos) {
            it(`PUT ${ruta} → 400 y no se reenvía`, async () => {
                const antes = algunaReenviada();
                const r = await pedir(puerto, 'PUT', ruta, USER);
                assert.equal(r.status, 400);
                assert.equal(algunaReenviada(), antes);
            });
        }

        for (const ruta of ['/empleados/docs/..;', '/empleados/docs/.;x', '/empleados/docs;x', '/empleados/openapi.json;x', '/empleados/docs/..%3B']) {
            it(`GET ${ruta} sin token → 400 (un ";" nunca pasa por documentación pública)`, async () => {
                const antes = algunaReenviada();
                const r = await pedir(puerto, 'GET', ruta, {});
                assert.equal(r.status, 400);
                assert.equal(algunaReenviada(), antes);
            });
        }

        it('el prefijo distingue mayúsculas: /EMPLEADOS → 404 y no se reenvía', async () => {
            const antes = algunaReenviada();
            const r = await pedir(puerto, 'GET', '/EMPLEADOS', ADMIN);
            assert.equal(r.status, 404);
            assert.equal(algunaReenviada(), antes);
        });

        it('una barra final no cuenta como el propio perfil', async () => {
            const r = await reenviada('/perfiles', 'PUT', '/perfiles/E001/', USER);
            assert.equal(r.status, 403);
        });

        it('un sub codificado se compara ya decodificado (E%30%301 = E001)', async () => {
            const r = await reenviada('/perfiles', 'PUT', '/perfiles/E%30%301', USER);
            assert.equal(r.status, 200);
            const r2 = await reenviada('/perfiles', 'PUT', '/perfiles/E%30%302', USER);
            assert.equal(r2.status, 403);
        });

        it('/empleadosX no es /empleados: 404 y no llega al servicio', async () => {
            const antes = algunaReenviada();
            const r = await pedir(puerto, 'GET', '/empleadosX', ADMIN);
            assert.equal(r.status, 404);
            assert.equal(algunaReenviada(), antes);
        });

        it('una ruta pública seguida de más segmentos ya no es pública', async () => {
            const r = await reenviada('/auth', 'POST', '/auth/login/extra', {});
            assert.equal(r.status, 401);
        });

        it('el prefijo de una ruta pública no cuenta (/auth/loginX)', async () => {
            const r = await reenviada('/auth', 'POST', '/auth/loginX', {});
            assert.equal(r.status, 401);
        });

        it('docs con más de un nivel ya no es público', async () => {
            const r = await reenviada('/empleados', 'GET', '/empleados/docs/a/b', {});
            assert.equal(r.status, 401);
        });
    });

    describe('Rutas públicas (D9): sin token', () => {
        for (const ruta of ['/auth/login', '/auth/recover-password', '/auth/reset-password']) {
            it(`POST ${ruta} se reenvía`, async () => {
                const r = await reenviada('/auth', 'POST', ruta, {});
                assert.equal(r.status, 200);
                assert.equal(r.reenvio.url, ruta);
            });
        }

        it('GET /auth/login no es público (solo POST)', async () => {
            const r = await reenviada('/auth', 'GET', '/auth/login', {});
            assert.equal(r.status, 401);
        });

        for (const prefijo of PREFIJOS) {
            for (const ruta of [`${prefijo}/docs`, `${prefijo}/docs/`, `${prefijo}/docs/swagger-ui.css`, `${prefijo}/openapi.json`]) {
                it(`GET ${ruta} se reenvía`, async () => {
                    const r = await reenviada(prefijo, 'GET', ruta, {});
                    assert.equal(r.status, 200);
                    assert.equal(r.reenvio.url, ruta);
                });
            }
        }

        it('la documentación no se puede modificar sin token', async () => {
            const r = await reenviada('/empleados', 'POST', '/empleados/docs', {});
            assert.equal(r.status, 401);
        });

        it('un token inválido enviado a una ruta pública se ignora', async () => {
            const r = await reenviada('/auth', 'POST', '/auth/login', { Authorization: 'Bearer basura' });
            assert.equal(r.status, 200);
        });
    });

    describe('Cabeceras de identidad (D10)', () => {
        const FALSAS = { 'X-User-Id': 'E999', 'x-user-role': 'ADMIN', 'X-USER-EMAIL': 'falso@empresa.com' };

        it('en una ruta pública se eliminan las que envía el cliente', async () => {
            const r = await reenviada('/auth', 'POST', '/auth/login', FALSAS);
            assert.equal(r.status, 200);
            for (const c of ['x-user-id', 'x-user-role', 'x-user-email']) {
                assert.equal(r.reenvio.headers[c], undefined, `${c} no debe reenviarse`);
            }
        });

        it('en una ruta protegida se reemplazan por la identidad verificada', async () => {
            const r = await reenviada('/empleados', 'GET', '/empleados', { ...FALSAS, ...USER });
            assert.equal(r.status, 200);
            assert.equal(r.reenvio.headers['x-user-id'], 'E001');
            assert.equal(r.reenvio.headers['x-user-role'], 'USER');
            assert.equal(r.reenvio.headers['x-user-email'], 'e1@empresa.com');
        });

        it('cabeceras X-User-Id repetidas se eliminan y se reemplazan por una sola', async () => {
            const r = await reenviada('/empleados', 'GET', '/empleados', { 'X-User-Id': ['E999', 'E888'], ...USER });
            assert.equal(r.status, 200);
            assert.equal(r.reenvio.headers['x-user-id'], 'E001');
        });

        it('una cabecera X-User-Role: ADMIN falsa no da permisos de escritura', async () => {
            const r = await reenviada('/empleados', 'DELETE', '/empleados/E002', { ...FALSAS, ...USER });
            assert.equal(r.status, 403);
        });

        it('sin token, las cabeceras falsas no autentican', async () => {
            const r = await reenviada('/empleados', 'GET', '/empleados', FALSAS);
            assert.equal(r.status, 401);
        });
    });

    describe('Comportamiento previo', () => {
        it('GET /health responde 200 sin token con el agregado de salud', async () => {
            const r = await pedir(puerto, 'GET', '/health');
            assert.equal(r.status, 200);
            assert.equal(r.cuerpo.origen, 'salud-de-prueba');
        });

        it('una ruta no enrutada responde 404 en JSON, con o sin token (Reto 3)', async () => {
            for (const cabeceras of [{}, USER]) {
                const r = await pedir(puerto, 'GET', '/no-existe', cabeceras);
                assert.equal(r.status, 404);
                assert.equal(r.cuerpo.error, 'Not Found');
                assert.equal(r.cuerpo.path, '/no-existe');
            }
        });

        it('si el servicio corta la respuesta a mitad, el Gateway no se cae y sigue atendiendo', async () => {
            const cortado = http.createServer((req, res) => {
                res.writeHead(200, { 'content-type': 'application/json', 'content-length': '100' });
                res.write('{"parcial":');
                setTimeout(() => res.socket.destroy(), 20);
            });
            const sano = servicioFalso();
            const pc = await escuchar(cortado);
            const ps = await escuchar(sano.servidor);
            const app = crearApp({
                jwtSecret: SECRETO,
                jwtIssuer: EMISOR,
                servicios: [
                    { nombre: 'cortado', prefijo: '/empleados', url: `http://127.0.0.1:${pc}` },
                    { nombre: 'sano', prefijo: '/perfiles', url: `http://127.0.0.1:${ps}` }
                ],
                salud: async () => ({})
            });
            const s = app.listen(0, '127.0.0.1');
            await new Promise((r) => s.once('listening', r));
            try {
                const resultado = await pedir(s.address().port, 'GET', '/empleados', ADMIN).then(
                    (r) => ({ ok: true, r }),
                    (e) => ({ ok: false, e })
                );
                // El cliente recibe la conexión cortada (no un 503 tras las cabeceras ni una espera infinita).
                assert.equal(resultado.ok, false, 'la respuesta incompleta debe cortarse');
                const siguiente = await pedir(s.address().port, 'GET', '/perfiles/E001', ADMIN);
                assert.equal(siguiente.status, 200);
            } finally {
                s.close();
                cortado.close();
                sano.servidor.close();
            }
        });

        it('un error inesperado responde 500 en JSON sin detalle interno', async () => {
            const app = crearApp({
                jwtSecret: SECRETO,
                jwtIssuer: EMISOR,
                servicios: [],
                salud: async () => {
                    throw new Error('detalle-interno-secreto');
                }
            });
            const s = app.listen(0, '127.0.0.1');
            await new Promise((r) => s.once('listening', r));
            try {
                const r = await pedir(s.address().port, 'GET', '/health');
                assert.equal(r.status, 500);
                assert.equal(r.cuerpo.error, 'Internal Server Error');
                assert.equal(JSON.stringify(r.cuerpo).includes('detalle-interno-secreto'), false);
                assert.equal(JSON.stringify(r.cuerpo).includes('at '), false);
            } finally {
                s.close();
            }
        });

        it('si el servicio no responde, 503 en JSON', async () => {
            const app = crearApp({
                jwtSecret: SECRETO,
                jwtIssuer: EMISOR,
                servicios: [{ nombre: 'caido', prefijo: '/empleados', url: 'http://127.0.0.1:1' }],
                salud: async () => ({})
            });
            const s = app.listen(0, '127.0.0.1');
            await new Promise((r) => s.once('listening', r));
            try {
                const r = await pedir(s.address().port, 'GET', '/empleados', ADMIN);
                assert.equal(r.status, 503);
                assert.equal(r.cuerpo.error, 'Service Unavailable');
            } finally {
                s.close();
            }
        });
    });
});
