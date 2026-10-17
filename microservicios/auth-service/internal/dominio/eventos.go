package dominio

// Tipos de evento (routing key = type). Catálogo de Eventos, secciones 3.1, 3.3 y 3.4 a 3.10.
const (
	// Consumidos
	EventoEmpleadoCreado        = "empleado.creado"
	EventoEmpleadoRetirado      = "empleado.retirado"
	EventoVacacionesIniciadas   = "vacaciones.iniciadas"
	EventoVacacionesFinalizadas = "vacaciones.finalizadas"
	// Publicados
	EventoUsuarioCreado       = "usuario.creado"
	EventoUsuarioRecuperacion = "usuario.recuperacion"
	EventoCuentaActivada      = "cuenta.activada"
	EventoCuentaDesactivada   = "cuenta.desactivada"
)

// EventosConsumidos son las routing keys que escucha la cola del auth-service.
var EventosConsumidos = []string{
	EventoEmpleadoCreado, EventoEmpleadoRetirado, EventoVacacionesIniciadas, EventoVacacionesFinalizadas,
}

// Motivos de cuenta.activada (catálogo 3.6) y de cuenta.desactivada (catálogo 3.7).
const (
	MotivoActivacionInicial = "ACTIVACION_INICIAL"
	MotivoFinVacaciones     = "FIN_VACACIONES"
	MotivoVacaciones        = "VACACIONES"
	MotivoRetiro            = "RETIRO"
)

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

// CuentaDesactivada es el payload de cuenta.desactivada (catálogo 3.7). Permanente
// distingue el retiro (true) de las vacaciones (false): no hay un evento aparte.
type CuentaDesactivada struct {
	EmpleadoID string `json:"empleadoId"`
	Email      string `json:"email"`
	Motivo     string `json:"motivo"`
	Permanente bool   `json:"permanente"`
}

// EmpleadoRetirado son los campos de empleado.retirado (catálogo 3.3) que usa el auth-service.
type EmpleadoRetirado struct {
	EmpleadoID string `json:"empleadoId"`
}

// VacacionesIniciadas son los campos de vacaciones.iniciadas (catálogo 3.9) que usa el auth-service.
type VacacionesIniciadas struct {
	VacacionesID string `json:"vacacionesId"`
	EmpleadoID   string `json:"empleadoId"`
}

// VacacionesFinalizadas son los campos de vacaciones.finalizadas (catálogo 3.10) que usa el auth-service.
type VacacionesFinalizadas struct {
	VacacionesID string `json:"vacacionesId"`
	EmpleadoID   string `json:"empleadoId"`
}
