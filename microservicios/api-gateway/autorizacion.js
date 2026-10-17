// Reglas de acceso del Gateway (Reto 5): matriz RBAC + propiedad del recurso (D8) y rutas
// públicas (D9). Todo se decide sobre los segmentos de la ruta ya analizada, nunca sobre
// el texto crudo, para que la misma ruta no pueda significar una cosa aquí y otra en el
// servicio que la recibe.

const METODOS_DE_LECTURA = new Set(['GET', 'HEAD']);

// Rutas de auth-service que no exigen token: sin token no se puede obtener un token.
const AUTH_PUBLICAS = new Set(['login', 'recover-password', 'reset-password']);

// Doble codificación de un separador, un punto o un ";" (%252F → "%2F" tras decodificar una
// vez): un servicio que decodificara otra vez vería "/", ".." o ";" donde el Gateway vio un nombre.
const DOBLE_CODIFICACION = /%(2f|5c|2e|3b)/i;

/**
 * Valida la ruta y la parte en segmentos decodificados.
 * Cada segmento se decodifica una vez y se rechaza si queda con "/" o "\\" (es decir,
 * venía como %2F o %5C), si es "." o ".." (también %2E%2E), si contiene ";" (también
 * %3B), si sigue conteniendo una secuencia %2F, %5C, %2E o %3B (doble codificación), si
 * está vacío en medio (//) o si la codificación es inválida.
 *
 * ";" se rechaza porque los servidores lo tratan distinto: Tomcat y Express descartan los
 * "parámetros de matriz" (/docs/..;x llega como /docs/..), así que una ruta que aquí parece
 * documentación pública podría resolverse en el servicio como otra distinta. Una barra final se conserva como un
 * segmento vacío al final, para que "/perfiles/E001/" no se confunda con "/perfiles/E001".
 *
 * @param {string} rutaCruda la ruta sin query string (req.path de Express, sin decodificar)
 * @returns {{ valida: true, segmentos: string[] } | { valida: false }}
 */
function analizarRuta(rutaCruda) {
    if (typeof rutaCruda !== 'string' || !rutaCruda.startsWith('/')) return { valida: false };

    const partes = rutaCruda.slice(1).split('/');
    const segmentos = [];
    for (let i = 0; i < partes.length; i++) {
        let segmento;
        try {
            segmento = decodeURIComponent(partes[i]);
        } catch {
            return { valida: false };
        }
        const esElUltimo = i === partes.length - 1;
        if (segmento === '' && !esElUltimo) return { valida: false };
        if (segmento === '.' || segmento === '..') return { valida: false };
        if (segmento.includes('/') || segmento.includes('\\')) return { valida: false };
        if (segmento.includes(';')) return { valida: false };
        if (DOBLE_CODIFICACION.test(segmento)) return { valida: false };
        segmentos.push(segmento);
    }
    return { valida: true, segmentos };
}

/** Prefijo ("/empleados") del servicio al que pertenece la ruta, o null. */
function servicioDeRuta(segmentos, prefijos) {
    const primero = `/${segmentos[0] ?? ''}`;
    return prefijos.includes(primero) ? primero : null;
}

/**
 * D9: documentación de cada servicio (GET/HEAD de /<servicio>/docs, sus recursos estáticos
 * /<servicio>/docs/<archivo> y /<servicio>/openapi.json) y los tres endpoints públicos de
 * auth-service (POST).
 */
function esRutaPublica(metodo, segmentos, prefijos) {
    if (!servicioDeRuta(segmentos, prefijos)) return false;
    const [servicio, recurso, ...resto] = segmentos;

    if (servicio === 'auth' && metodo === 'POST' && resto.length === 0 && AUTH_PUBLICAS.has(recurso)) {
        return true;
    }
    if (!METODOS_DE_LECTURA.has(metodo)) return false;
    if (recurso === 'openapi.json' && resto.length === 0) return true;
    // /x/docs, /x/docs/ y /x/docs/<archivo> (un solo nivel: así sirve sus recursos swagger-ui-express).
    if (recurso === 'docs' && resto.length <= 1) return true;
    return false;
}

/**
 * D8: ¿puede este usuario hacer esta petición protegida?
 * - ADMIN: todo.
 * - USER: lectura (GET/HEAD) de cualquier recurso; POST /auth/change-password;
 *   PUT /perfiles/{empleadoId} solo si empleadoId es su propio sub. Lo demás no.
 */
function decidirAcceso(usuario, metodo, segmentos) {
    if (usuario.role === 'ADMIN') return true;
    if (usuario.role !== 'USER') return false;

    if (METODOS_DE_LECTURA.has(metodo)) return true;

    const [servicio, recurso, ...resto] = segmentos;
    if (metodo === 'POST' && servicio === 'auth' && recurso === 'change-password' && resto.length === 0) {
        return true;
    }
    if (metodo === 'PUT' && servicio === 'perfiles' && resto.length === 0 && recurso !== undefined && recurso !== '') {
        return recurso === usuario.sub;
    }
    return false;
}

module.exports = { analizarRuta, servicioDeRuta, esRutaPublica, decidirAcceso };
