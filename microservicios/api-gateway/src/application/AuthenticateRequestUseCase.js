const UnauthorizedError = require('../domain/exceptions/UnauthorizedError');

class AuthenticateRequestUseCase {
    async execute({ user, method, path, params = {} }) {
        if (!user || !user.roles) {
            throw new UnauthorizedError('No autorizado. Token inválido o no proporcionado.');
        }

        const roles = Array.isArray(user.roles) ? user.roles : [user.roles];
        const upperMethod = (method || 'GET').toUpperCase();
        const isGet = upperMethod === 'GET';

        if (roles.includes('ADMIN')) {
            return true;
        }

        if (roles.includes('USER')) {
            if (upperMethod === 'POST' && path === '/auth/change-password') {
                return true;
            }

            if (upperMethod === 'PUT' && path.startsWith('/perfiles/')) {
                const targetEmpleadoId = params.empleadoId || path.split('/perfiles/')[1]?.split('/')[0];

                if (String(user.userId) === String(targetEmpleadoId)) {
                    return true;
                }

                throw new UnauthorizedError('No autorizado. No puede modificar el perfil de otro usuario.');
            }

            if (isGet) {
                return true;
            }

            throw new UnauthorizedError('No autorizado. No tiene permisos para realizar esta acción.');
        }

        throw new UnauthorizedError('No autorizado. No tiene permisos para realizar esta acción.');
    }
}

module.exports = AuthenticateRequestUseCase;