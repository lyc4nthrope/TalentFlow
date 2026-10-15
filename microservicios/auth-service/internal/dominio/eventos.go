package dominio

// Tipos de evento (routing key = type). Catálogo de Eventos, secciones 3.1 y 3.4 a 3.6.
const (
	EventoEmpleadoCreado      = "empleado.creado"
	EventoUsuarioCreado       = "usuario.creado"
	EventoUsuarioRecuperacion = "usuario.recuperacion"
	EventoCuentaActivada      = "cuenta.activada"
)

// Motivos de cuenta.activada (catálogo 3.6).
const MotivoActivacionInicial = "ACTIVACION_INICIAL"

// UsuarioCreado es el payload de usuario.creado (catálogo 3.4). Nunca lleva una
// contraseña: el empleado establece la suya con el token.
type UsuarioCreado struct {
	EmpleadoID      string `json:"empleadoId"`
	Email           string `json:"email"`
	TokenActivacion string `json:"tokenActivacion"`
	ExpiraEn        string `json:"expiraEn"`
}

// UsuarioRecuperacion es el payload de usuario.recuperacion (catálogo 3.5).
type UsuarioRecuperacion struct {
	Email             string `json:"email"`
	TokenRecuperacion string `json:"tokenRecuperacion"`
	ExpiraEn          string `json:"expiraEn"`
}

// CuentaActivada es el payload de cuenta.activada (catálogo 3.6).
type CuentaActivada struct {
	EmpleadoID string `json:"empleadoId"`
	Email      string `json:"email"`
	Motivo     string `json:"motivo"`
}

// EmpleadoCreado son los campos de empleado.creado (catálogo 3.1) que usa el auth-service.
type EmpleadoCreado struct {
	EmpleadoID string `json:"empleadoId"`
	Email      string `json:"email"`
}
