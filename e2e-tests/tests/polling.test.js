'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { eventualmente, nuncaDurante, idUnico, emailUnico } = require('../support/polling');

const rapido = { intentos: 3, intervaloMs: 1 };

test('devuelve el primer valor verdadero sin agotar los intentos', async () => {
  let llamadas = 0;
  const valor = await eventualmente(() => (++llamadas === 2 ? { id: 'N-1' } : null), rapido);
  assert.deepEqual(valor, { id: 'N-1' });
  assert.equal(llamadas, 2);
});

test('acepta una condición asíncrona y reintenta cuando lanza', async () => {
  let llamadas = 0;
  const valor = await eventualmente(async () => {
    llamadas++;
    if (llamadas < 3) throw new Error('todavía no');
    return 'listo';
  }, rapido);
  assert.equal(valor, 'listo');
  assert.equal(llamadas, 3);
});

test('si se agota, el mensaje trae descripción, intentos, tiempo y el último valor', async () => {
  let llamadas = 0;
  await assert.rejects(
    eventualmente(() => (++llamadas, null), { ...rapido, descripcion: 'la notificación llega' }),
    /^Error: No se cumplió "la notificación llega" después de 3 intentos \(\d+ ms\)\. Último valor observado: null$/
  );
  assert.equal(llamadas, 3);
});

test('si el último intento lanzó, el mensaje muestra ese error y no uno anterior', async () => {
  let llamadas = 0;
  await assert.rejects(
    eventualmente(() => {
      throw new Error(`HTTP 50${++llamadas}`);
    }, rapido),
    /Último valor observado: error: HTTP 503$/
  );
});

test('un último valor undefined también se informa', async () => {
  await assert.rejects(
    eventualmente(() => [].find(Boolean), { intentos: 1, intervaloMs: 0 }),
    /después de 1 intentos .*Último valor observado: undefined$/
  );
});

test('espera el intervalo solo entre intentos, no después del último', async () => {
  const inicio = Date.now();
  await assert.rejects(eventualmente(() => false, { intentos: 3, intervaloMs: 100 }));
  const transcurrido = Date.now() - inicio;
  assert.ok(transcurrido >= 195, `esperó ${transcurrido} ms; se esperaban 2 intervalos`);
  assert.ok(transcurrido < 290, `esperó ${transcurrido} ms; sobra una espera`);
});

test('rechaza parámetros inválidos antes de llamar a la condición', async () => {
  let llamadas = 0;
  const condicion = () => ++llamadas;
  await assert.rejects(eventualmente(condicion, { intentos: 0, intervaloMs: 1 }), /"intentos"/);
  await assert.rejects(eventualmente(condicion, { intentos: 2, intervaloMs: -1 }), /"intervaloMs"/);
  await assert.rejects(eventualmente(condicion), /"intentos"/);
  assert.equal(llamadas, 0);
});

test('nuncaDurante vigila toda la ventana y observa también al final', async () => {
  let llamadas = 0;
  const inicio = Date.now();
  const resultado = await nuncaDurante(() => (++llamadas, null), { duracionMs: 100, intervaloMs: 40 });
  const transcurrido = Date.now() - inicio;
  assert.ok(transcurrido >= 100, `vigiló ${transcurrido} ms; se esperaba la ventana completa`);
  // Al empezar, a los 40 y 80 ms y al cumplirse los 100 ms.
  assert.equal(llamadas, 4);
  assert.equal(resultado.observaciones, 4);
});

test('nuncaDurante falla en cuanto observa lo prohibido, con la descripción y lo que vio', async () => {
  let llamadas = 0;
  await assert.rejects(
    nuncaDurante(() => (++llamadas === 2 ? 'llegó un correo de recuperación' : null), {
      duracionMs: 1000,
      intervaloMs: 5,
      descripcion: 'un token nuevo',
    }),
    (error) => {
      assert.match(error.message, /^Se observó "un token nuevo" a los \d+ ms de una vigilancia de 1000 ms \(observación 2\)/);
      assert.match(error.message, /llegó un correo de recuperación$/);
      return true;
    }
  );
  assert.equal(llamadas, 2);
});

test('nuncaDurante no convierte un error de la consulta en una ausencia', async () => {
  await assert.rejects(
    nuncaDurante(() => Promise.reject(new Error('GET /notificaciones devolvió 500')), { duracionMs: 50, intervaloMs: 5 }),
    /devolvió 500/
  );
});

test('nuncaDurante rechaza una ventana o un intervalo que no son enteros positivos', async () => {
  await assert.rejects(nuncaDurante(() => null, { duracionMs: 0, intervaloMs: 5 }), /"duracionMs"/);
  await assert.rejects(nuncaDurante(() => null, { duracionMs: 10, intervaloMs: 0 }), /"intervaloMs"/);
});

test('idUnico sigue el formato de D3 y no se repite', () => {
  const ids = new Set(Array.from({ length: 1000 }, () => idUnico()));
  assert.equal(ids.size, 1000);
  for (const id of ids) assert.match(id, /^E-[0-9a-z]{8}-[0-9a-f]{6}$/);
  assert.match(idUnico('D'), /^D-[0-9a-z]{8}-[0-9a-f]{6}$/);
  // Cabe en los VARCHAR(20) de las BD de empleados, departamentos y vacaciones.
  assert.ok(idUnico('D').length <= 20);
});

test('emailUnico produce direcciones distintas, en minúsculas y con el dominio pedido', () => {
  const a = emailUnico('Ana');
  const b = emailUnico('Ana');
  assert.notEqual(a, b);
  assert.match(a, /^ana-[0-9a-z]{8}-[0-9a-f]{6}@empresa\.com$/);
  assert.match(emailUnico('x', 'prueba.co'), /@prueba\.co$/);
});
