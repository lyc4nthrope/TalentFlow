const AuthenticateRequestUseCase = require('../../application/usecases/AuthenticateRequestUseCase');
const JwtTokenVerifierAdapter = require('../../domain/ports/TokenVerifierPort');
const UnauthorizedError = require('../../domain/exceptions/UnauthorizedError');

const tokenVerifier = new JwtTokenVerifierAdapter(process.env.JWT_SECRET);
const authenticateRequestUseCase = new AuthenticateRequestUseCase(tokenVerifier);

async function authMiddleware(req, res, next) {
    try {
        const authHeader = req.headers.authorization;

        const payload = await authenticateRequestUseCase.execute(authHeader);

        // 1. Guarda el usuario en el request local
        req.user = payload;

        // 2/ Inyectamos la cabecera de autorización en la solicitud para que los microservicios puedan verificarla
        req.headers['x-user-id'] = payload.userId;
        req.headers['x-user-email'] = payload.email;
        req.headers['x-user-roles'] = Array.isArray(payload.roles)
            ? payload.roles.join(',')
            : (payload.roles || payload.rol || '');

        next();
    } catch (error) {
        if (error instanceof UnauthorizedError || error.name === 'Token') {
            return res.status(401).json({
                error: 'UNAUTHORIZED',
                message: error.message || 'No autorizado. Token inválido o no proporcionado.'
            });
        }

        return res.status(500).json({
            error: 'INTERNAL_SERVER_ERROR',
            message: 'Error interno del servidor'
        });
    }
}

module.exports = authMiddleware;