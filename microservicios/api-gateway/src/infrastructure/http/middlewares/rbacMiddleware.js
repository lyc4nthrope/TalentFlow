const AuthorizeRequestUseCase = require("../../../application/usecases/AuthorizeRequestUseCase");
const ForbiddenError = require("../../../domain/exceptions/ForbiddenError");

const authorizeUseCase = new AuthorizeRequestUseCase();

function authorizeMiddleware(req, res, next) {
    try {
        authorizeUseCase.execute({
            user: req.user,
            method: req.method,
            path: req.baseUrl +req.path,
            params: req.params
        });

        next();
    } catch (error) {
        if (error instanceof ForbiddenError) {
            return res.status(403).json({
                error: 'FORBIDDEN',
                message: error.message || 'No tiene permisos para realizar esta acción.'
            });
        }

        return res.status(500).json({
            error: 'INTERNAL_SERVER_ERROR',
            message: 'Error interno del servidor'
        });
    }
}

module.exports = authorizeMiddleware;