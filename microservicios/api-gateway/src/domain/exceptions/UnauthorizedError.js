class UnauthorizedError extends Error {
    constructor(message = "No autorizado. Token Invalido o no proporcionado.") {
        super(message);
        this.name = "UnauthorizedError";
        this.statusCode = 401; // Código de estado HTTP para "No autorizado"
    }
}

module.exports = UnauthorizedError;