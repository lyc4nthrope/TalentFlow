const { describe, it } = require('node:test');
const assert = require('node:assert/strict');

const { analizarRuta, servicioDeRuta, esRutaPublica, decidirAcceso } = require('../autorizacion');

// Pruebas directas de las reglas de acceso. gateway.test.js las recorre a través de HTTP;
// aquí se cubren las defensas que la cadena de middlewares nunca deja llegar (la ruta
// siempre es un string con "/", el 404 se decide antes, el token ya trae un rol válido),
// para que cada función siga siendo segura usada por sí sola.

const PREFIJOS = ['/auth', '/empleados', '/perfiles'];

describe('analizarRuta', () => {
    it('rechaza lo que no es una ruta absoluta', () => {
        for (const ruta of [undefined, null, 42, '', 'empleados/E001']) {
            assert.deepEqual(analizarRuta(ruta), { valida: false }, `ruta ${String(ruta)}`);
        }
    });

    it('parte una ruta válida en segmentos decodificados', () => {
        assert.deepEqual(analizarRuta('/empleados/E%20001'), { valida: true, segmentos: ['empleados', 'E 001'] });
    });
});

describe('servicioDeRuta', () => {
    it('sin segmentos no hay servicio', () => {
        assert.equal(servicioDeRuta([], PREFIJOS), null);
    });

    it('devuelve el prefijo del servicio dueño de la ruta', () => {
        assert.equal(servicioDeRuta(['empleados', 'E001'], PREFIJOS), '/empleados');
    });
});

describe('esRutaPublica', () => {
    it('una ruta que no es de ningún servicio nunca es pública, aunque parezca documentación', () => {
        assert.equal(esRutaPublica('GET', ['otro', 'docs'], PREFIJOS), false);
        assert.equal(esRutaPublica('POST', ['login'], PREFIJOS), false);
    });
});

describe('decidirAcceso', () => {
    it('niega todo a un rol que no es ADMIN ni USER, incluso la lectura', () => {
        for (const role of ['AUDITOR', undefined, 'admin']) {
            assert.equal(decidirAcceso({ sub: 'E001', role }, 'GET', ['empleados']), false, `rol ${role}`);
        }
    });
});
