class AppError extends Error {
  constructor(mensaje, codigoEstado = 400, errores = []) {
    super(mensaje);
    this.name = "AppError";
    this.codigoEstado = codigoEstado;
    this.errores = errores;
  }
}

module.exports = { AppError };
