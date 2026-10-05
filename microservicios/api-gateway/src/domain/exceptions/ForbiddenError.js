class ForbbidenError extends Error {
    constructor(message = "Acceso prohibido. No tiene permisos para acceder a este recurso.") {
        super(message);
        this.name = "ForbbidenError";
        this.statusCode = 403; // Código de estado HTTP para "Prohibido"
    }
}

module.exports = ForbbidenError;