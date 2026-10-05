const UnauthorizedError = require('../exceptions/UnauthorizedError');

class AuthenticateRequestUseCase {
    /**
     * @param {object} user
     * @param {string} method
     * @param {string} path
     * @param {object} params
     */

    async execute({user, method, path, params = {}}) {
        const roles = user.roles || [];
        const isMethodGet = method.toUpperCase() === 'GET';

        // 1. Regla ADMIN
        if(roles,includes('ADMIN')) {
            return true;
        }

        // 2. Regla USER
        if(roles.includes('USER')) {
            // Excepcion 1: POST /auth/change-password
            if(method.toUpperCase() === 'POST' && path === '/auth/change-password') {
                return true;
            }

            // Excepcion 2: PUT /perfiles/{empeladoId}
            if(method.toUpperCase() === 'PUT' && path.startsWith('/perfiles/')) {
                const targetEmpeladoId = params.empleadoId;

                if(user.userId === targetEmpeladoId) {
                    return true;
                }
                throw new UnauthorizedError('No autorizado. No puede modificar el perfil de otro usuario.');
            }

            // Acceso general
            if (isMethodGet) {
                return true;
            }

            throw new ForbiddenError('No autorizado. No tiene permisos para realizar esta acción.');
        }

        // 3. Otros roles
        throw new UnauthorizedError('No autorizado. No tiene permisos para realizar esta acción.');
    }
}

module.exports = AuthenticateRequestUseCase;