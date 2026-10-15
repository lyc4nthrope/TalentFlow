const jwt = require('jsonwebtoken');

const ROLES = new Set(['ADMIN', 'USER']);

/** Extrae el token de una cabecera "Authorization: Bearer <token>"; null si no hay. */
function leerBearer(cabecera) {
    if (typeof cabecera !== 'string') return null;
    const coincidencia = /^Bearer\s+(\S+)$/i.exec(cabecera.trim());
    return coincidencia ? coincidencia[1] : null;
}

/**
 * Valida el token de acceso del auth-service (Reto 5, D2) y devuelve su identidad.
 * Solo HS256 (fijado: un token firmado con otro algoritmo y la misma clave no pasa), del
 * emisor esperado y con exp (jsonwebtoken acepta tokens sin exp como si no vencieran).
 * Lanza si el token no es válido.
 *
 * @returns {{ sub: string, role: 'ADMIN' | 'USER', email: string | undefined }}
 */
function verificarToken(token, { secreto, emisor }) {
    const carga = jwt.verify(token, secreto, { algorithms: ['HS256'], issuer: emisor });
    if (typeof carga.exp !== 'number') {
        throw new Error('token sin exp');
    }
    if (typeof carga.sub !== 'string' || carga.sub === '') {
        throw new Error('token sin sub');
    }
    if (!ROLES.has(carga.role)) {
        throw new Error('token con un rol desconocido');
    }
    return { sub: carga.sub, role: carga.role, email: carga.email };
}

module.exports = { leerBearer, verificarToken };
