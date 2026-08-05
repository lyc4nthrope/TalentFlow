class AppError extends Error {
  constructor(mensaje, codigoEstado = 400) {
    super(mensaje);
    this.name = "AppError";
    this.codigoEstado = codigoEstado;
  }
}

module.exports = { AppError };
