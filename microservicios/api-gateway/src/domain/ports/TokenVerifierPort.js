/** 
 * Puerto de verfiicación de token
 * define los métodos que debe implementar un verificador de token
 */
const UnauthorizedError = require('../exceptions/UnauthorizedError');
const TokenVerifierPort = require('./TokenVerifierPort');
const jwt = require('jsonwebtoken');
class JwtTokenVerifierAdapter extends TokenVerifierPort {

    /**
     * @param {string} clave - La clave utilizada para verificar la firma del token
     */
    constructor(clave) {
        super();
        if (!clave) {
            throw new UnauthorizedError('Clave no proporcionada');
        }
        this.clave = clave;
    }

    /**
     * Verifica la validez de un token JWT
     * @param {string} token - El token JWT a verificar
     * @returns {boolean} - true si el token es válido, false en caso contrario
     * @throws {UnauthorizedError} - Si el token es inválido o ha expirado
     */
    async verify(token) {

        try {

            // jwt.verifi valida la firma, expiracion y vigenia
            const decode = jwt.verify(token, this.clave);

            //Mapeo de la respuesta
            return {
                userId: decode.sub || decode.id || decode.userId,
                email:decode.email,
                roles: decode.roles || []
            };
        } catch (error) {
            // Manejo de errores específicos de JWT
            if(error.name === 'TokenExpiredError') {
                throw new UnauthorizedError('Token expirado');
            }
            if(error.name === 'JsonWebTokenError') {
                throw new UnauthorizedError('Token inválido');
            }
            // Otros errores
            throw new UnauthorizedError('Error al verificar el token: ' + error.message);   
        }
    }

    /**
     * Extrae los datos (payload) del token sin validar la firma
     * @param {string} token - El token JWT del cual extraer los datos
     * @returns {object} - Los datos extraídos del token
     * @throws {Error} - Si el token es inválido o no se puede decodificar
     */
    decode(token) {
        return jwt.decode(token);
    }
}
module.exports = JwtTokenVerifierAdapter;