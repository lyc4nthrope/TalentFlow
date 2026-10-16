package aplicacion

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/eventos"
)

// Opciones de los casos de uso.
type Opciones struct {
	VigenciaActivacion   time.Duration // token de activación (alta del empleado)
	VigenciaRecuperacion time.Duration // token de recuperación de contraseña
	CostoBcrypt          int
}

// AuthService implementa los casos de uso de cuentas y tokens (Reto 5, F2).
type AuthService struct {
	repo       Repositorio
	publicador Publicador
	emisor     EmisorTokens
	generar    GeneradorTokens
	ahora      Reloj
	opciones   Opciones
	// hashSenuelo se compara cuando el usuario no existe, para que la respuesta tarde
	// lo mismo que con un usuario real (no revela qué emails existen por el tiempo).
	hashSenuelo []byte
}

func NuevoAuthService(repo Repositorio, publicador Publicador, emisor EmisorTokens,
	generar GeneradorTokens, ahora Reloj, opciones Opciones) (*AuthService, error) {
	if opciones.CostoBcrypt == 0 {
		opciones.CostoBcrypt = bcrypt.DefaultCost
	}
	senuelo, err := bcrypt.GenerateFromPassword([]byte("senuelo-sin-uso-Aa1"), opciones.CostoBcrypt)
	if err != nil {
		return nil, fmt.Errorf("preparar hash señuelo: %w", err)
	}
	return &AuthService{repo: repo, publicador: publicador, emisor: emisor, generar: generar,
		ahora: ahora, opciones: opciones, hashSenuelo: senuelo}, nil
}

func normalizarEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Límites de las columnas de usuarios (schema.sql).
const (
	longitudMaximaEmpleadoID = 36
	longitudMaximaEmail      = 255
)

// emailConFormato es una comprobación básica: una sola @, con algo antes y un
// dominio con punto después, sin espacios.
func emailConFormato(email string) bool {
	local, dominioEmail, ok := strings.Cut(email, "@")
	return ok && local != "" && strings.Contains(dominioEmail, ".") &&
		!strings.ContainsAny(dominioEmail, "@") && !strings.ContainsAny(email, " \t\r\n") &&
		!strings.HasPrefix(dominioEmail, ".") && !strings.HasSuffix(dominioEmail, ".")
}

// CrearCuentaDesdeEmpleado reacciona a empleado.creado: crea la cuenta en
// PENDIENTE_ACTIVACION, sin contraseña, con rol USER y un token de activación, y
// publica usuario.creado.
//   - El id del evento se registra en la misma transacción que la cuenta (D4): un
//     mensaje repetido no crea nada ni vuelve a publicar.
//   - Si la cuenta del empleado ya existe, no hace nada.
//   - Si el email ya pertenece a otra cuenta, o los datos no caben o no tienen
//     formato válido, devuelve ErrSolicitudInvalida / ErrRechazoPermanente: el
//     consumidor descarta el mensaje, porque reintentarlo nunca funcionaría.
func (s *AuthService) CrearCuentaDesdeEmpleado(ctx context.Context, eventoID string, e dominio.EmpleadoCreado) error {
	empleadoID := strings.TrimSpace(e.EmpleadoID)
	email := normalizarEmail(e.Email)
	switch {
	case empleadoID == "" || email == "":
		return fmt.Errorf("%w: empleado.creado sin empleadoId o email", ErrSolicitudInvalida)
	case len(empleadoID) > longitudMaximaEmpleadoID:
		return fmt.Errorf("%w: empleadoId de más de %d caracteres", ErrSolicitudInvalida, longitudMaximaEmpleadoID)
	case len(email) > longitudMaximaEmail || !emailConFormato(email):
		return fmt.Errorf("%w: email sin formato válido", ErrSolicitudInvalida)
	case empleadoID == dominio.SujetoAdmin:
		// El "sub" del administrador semilla está reservado: una cuenta de empleado
		// con ese id compartiría la identidad del administrador en el JWT.
		log.Printf("auth-service: no se crea la cuenta: el empleadoId %q está reservado para el administrador", empleadoID)
		return fmt.Errorf("%w: empleadoId reservado para el administrador", ErrRechazoPermanente)
	}
	if err := validarEventoID(eventoID); err != nil {
		return err
	}

	var aPublicar *dominio.UsuarioCreado
	duplicado, err := s.repo.ProcesarUnaVez(ctx, eventoID, func(repo Repositorio) error {
		if hecho, err := cuentaYaResuelta(ctx, repo, empleadoID, email); hecho || err != nil {
			return err
		}
		token, err := s.generar()
		if err != nil {
			return fmt.Errorf("generar token de activación: %w", err)
		}
		expiraEn := s.ahora().UTC().Add(s.opciones.VigenciaActivacion)
		usuario := &dominio.Usuario{
			Sujeto:        empleadoID,
			EmpleadoID:    empleadoID,
			Email:         email,
			Rol:           dominio.RolUser,
			Estado:        dominio.EstadoPendienteActivacion,
			TokenHash:     dominio.HashToken(token),
			TokenTipo:     dominio.TokenActivacion,
			TokenExpiraEn: expiraEn,
		}
		if err := repo.Crear(ctx, usuario); err != nil {
			if errors.Is(err, ErrUsuarioDuplicado) {
				// Otra creación ganó la carrera: se vuelve a mirar quién es el dueño.
				hecho, err := cuentaYaResuelta(ctx, repo, empleadoID, email)
				if !hecho && err == nil {
					// El choque no es con la cuenta del empleado ni con su email (p. ej. el
					// sujeto de otra cuenta): no se puede crear y reintentarlo no cambia nada.
					log.Printf("auth-service: no se crea la cuenta del empleado %s: choca con una cuenta existente que no es la suya", empleadoID)
					return fmt.Errorf("%w: el empleadoId choca con otra cuenta", ErrRechazoPermanente)
				}
				return err
			}
			return fmt.Errorf("crear cuenta: %w", err)
		}
		aPublicar = &dominio.UsuarioCreado{
			EmpleadoID:      empleadoID,
			Email:           email,
			TokenActivacion: token,
			ExpiraEn:        expiraEn.Format(time.RFC3339),
		}
		return nil
	})
	if duplicado {
		log.Printf("auth-service: evento duplicado descartado: %s ya se había procesado (empleado.creado)", eventoID)
		return nil
	}
	if err != nil {
		return err
	}
	if aPublicar != nil {
		// Paso 3 del reto5.pdf: el log debe mostrar que se detectó empleado.creado y se generó el usuario.
		log.Printf("auth-service: empleado.creado recibido (id %s): cuenta del empleado %s creada en %s",
			eventoID, empleadoID, dominio.EstadoPendienteActivacion)
		s.publicar(ctx, dominio.EventoUsuarioCreado, *aPublicar)
	}
	return nil
}

// cuentaYaResuelta indica si no hay nada que crear: la cuenta del empleado ya
// existe (true, nil) o el email pertenece a otra cuenta (true, ErrRechazoPermanente).
func cuentaYaResuelta(ctx context.Context, repo Repositorio, empleadoID, email string) (bool, error) {
	propia, err := repo.BuscarPorEmpleadoID(ctx, empleadoID)
	if err != nil {
		return true, fmt.Errorf("buscar cuenta del empleado: %w", err)
	}
	if propia != nil {
		log.Printf("auth-service: la cuenta del empleado %s ya existe; no se crea otra", empleadoID)
		return true, nil
	}
	conEseEmail, err := repo.BuscarPorEmail(ctx, email)
	if err != nil {
		return true, fmt.Errorf("buscar cuenta por email: %w", err)
	}
	if conEseEmail != nil {
		log.Printf("auth-service: no se crea la cuenta del empleado %s: el email ya pertenece a otra cuenta (%s)",
			empleadoID, conEseEmail.Sujeto)
		return true, fmt.Errorf("%w: el email ya pertenece a otra cuenta", ErrRechazoPermanente)
	}
	return false, nil
}

// Login devuelve un token de acceso solo para cuentas ACTIVA con la contraseña
// correcta. Cualquier otro caso responde lo mismo: credenciales inválidas.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	email = normalizarEmail(email)
	if email == "" || password == "" {
		return "", ErrCredencialesInvalidas
	}
	usuario, err := s.repo.BuscarPorEmail(ctx, email)
	if err != nil {
		return "", fmt.Errorf("buscar cuenta: %w", err)
	}
	if usuario == nil || usuario.PasswordHash == "" {
		_ = bcrypt.CompareHashAndPassword(s.hashSenuelo, []byte(password))
		return "", ErrCredencialesInvalidas
	}
	if bcrypt.CompareHashAndPassword([]byte(usuario.PasswordHash), []byte(password)) != nil {
		return "", ErrCredencialesInvalidas
	}
	if !usuario.PuedeAutenticarse() {
		return "", ErrCredencialesInvalidas
	}
	token, err := s.emisor.Emitir(usuario.Sujeto, usuario.Rol, usuario.Email)
	if err != nil {
		return "", fmt.Errorf("emitir token de acceso: %w", err)
	}
	return token, nil
}

// SolicitarRecuperacion genera un token de recuperación (reemplaza al anterior) y
// publica usuario.recuperacion. Para un email desconocido o una cuenta retirada no
// hace nada y tampoco falla: el handler responde 202 siempre (D16).
func (s *AuthService) SolicitarRecuperacion(ctx context.Context, email string) error {
	email = normalizarEmail(email)
	if email == "" {
		return nil
	}
	usuario, err := s.repo.BuscarPorEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("buscar cuenta: %w", err)
	}
	if usuario == nil || usuario.Estado == dominio.EstadoDesactivadaPermanente {
		return nil
	}

	token, err := s.generar()
	if err != nil {
		return fmt.Errorf("generar token de recuperación: %w", err)
	}
	expiraEn := s.ahora().UTC().Add(s.opciones.VigenciaRecuperacion)
	if err := s.repo.GuardarToken(ctx, usuario.ID, dominio.HashToken(token), dominio.TokenRecuperacion, expiraEn); err != nil {
		return fmt.Errorf("guardar token de recuperación: %w", err)
	}

	s.publicar(ctx, dominio.EventoUsuarioRecuperacion, dominio.UsuarioRecuperacion{
		Email:             usuario.Email,
		TokenRecuperacion: token,
		ExpiraEn:          expiraEn.Format(time.RFC3339),
	})
	return nil
}

// RestablecerPassword fija la contraseña con un token de activación o recuperación
// vigente y lo consume (un solo uso). Solo una cuenta PENDIENTE_ACTIVACION pasa a
// ACTIVA (y publica cuenta.activada); una suspendida sigue suspendida y una retirada
// se rechaza.
func (s *AuthService) RestablecerPassword(ctx context.Context, token, nueva string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrTokenInvalido
	}
	if err := dominio.ValidarPassword(nueva); err != nil {
		return err
	}

	tokenHash := dominio.HashToken(token)
	usuario, err := s.repo.BuscarPorTokenHash(ctx, tokenHash)
	if err != nil {
		return fmt.Errorf("buscar token: %w", err)
	}
	if usuario == nil || !s.tokenVigente(usuario) || usuario.Estado == dominio.EstadoDesactivadaPermanente {
		return ErrTokenInvalido
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(nueva), s.opciones.CostoBcrypt)
	if err != nil {
		return fmt.Errorf("generar hash: %w", err)
	}
	activar := usuario.Estado == dominio.EstadoPendienteActivacion
	estado := usuario.Estado
	if activar {
		estado = dominio.EstadoActiva
	}
	consumido, err := s.repo.ConsumirToken(ctx, usuario.ID, tokenHash, usuario.Estado, string(hash), estado)
	if err != nil {
		return fmt.Errorf("consumir token: %w", err)
	}
	if !consumido {
		return ErrTokenInvalido
	}

	if activar {
		s.publicar(ctx, dominio.EventoCuentaActivada, dominio.CuentaActivada{
			EmpleadoID: usuario.EmpleadoID,
			Email:      usuario.Email,
			Motivo:     dominio.MotivoActivacionInicial,
		})
	}
	return nil
}

func (s *AuthService) tokenVigente(u *dominio.Usuario) bool {
	tipoValido := u.TokenTipo == dominio.TokenActivacion || u.TokenTipo == dominio.TokenRecuperacion
	return tipoValido && s.ahora().Before(u.TokenExpiraEn)
}

// CambiarPassword cambia la contraseña de quien presenta un token de acceso válido.
// La cuenta debe estar ACTIVA y la contraseña actual debe coincidir.
func (s *AuthService) CambiarPassword(ctx context.Context, tokenAcceso, actual, nueva string) error {
	identidad, err := s.emisor.Validar(tokenAcceso)
	if err != nil {
		return ErrNoAutenticado
	}
	if actual == "" {
		return ErrPasswordActual
	}
	if err := dominio.ValidarPassword(nueva); err != nil {
		return err
	}
	usuario, err := s.repo.BuscarPorSujeto(ctx, identidad.Sujeto)
	if err != nil {
		return fmt.Errorf("buscar cuenta: %w", err)
	}
	if usuario == nil || !usuario.PuedeAutenticarse() {
		return ErrNoAutenticado
	}
	if bcrypt.CompareHashAndPassword([]byte(usuario.PasswordHash), []byte(actual)) != nil {
		return ErrPasswordActual
	}
	if actual == nueva {
		return ErrPasswordRepetida
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(nueva), s.opciones.CostoBcrypt)
	if err != nil {
		return fmt.Errorf("generar hash: %w", err)
	}
	if err := s.repo.ActualizarPassword(ctx, usuario.ID, string(hash)); err != nil {
		return fmt.Errorf("actualizar contraseña: %w", err)
	}
	return nil
}

// SembrarAdmin crea el administrador semilla (D15) si se configuró y aún no existe.
// La contraseña nunca se escribe en el log.
func (s *AuthService) SembrarAdmin(ctx context.Context, email, password string) error {
	email = normalizarEmail(email)
	if email == "" || password == "" {
		log.Printf("auth-service: ADMIN_EMAIL/ADMIN_PASSWORD no configurados; no se crea el administrador semilla")
		return nil
	}
	if err := dominio.ValidarPassword(password); err != nil {
		log.Printf("auth-service: ADMIN_PASSWORD no cumple la política de contraseñas; no se crea el administrador semilla")
		return nil
	}
	existente, err := s.repo.BuscarPorSujeto(ctx, dominio.SujetoAdmin)
	if err != nil {
		return fmt.Errorf("buscar administrador: %w", err)
	}
	if existente != nil {
		log.Printf("auth-service: el administrador semilla ya existe")
		return nil
	}
	conEseEmail, err := s.repo.BuscarPorEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("buscar cuenta por email: %w", err)
	}
	if conEseEmail != nil {
		log.Printf("auth-service: ADMIN_EMAIL ya pertenece a una cuenta que no es el administrador (%s); no se crea el administrador semilla",
			conEseEmail.Sujeto)
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.opciones.CostoBcrypt)
	if err != nil {
		return fmt.Errorf("generar hash del administrador: %w", err)
	}
	err = s.repo.Crear(ctx, &dominio.Usuario{
		Sujeto:       dominio.SujetoAdmin,
		Email:        email,
		PasswordHash: string(hash),
		Rol:          dominio.RolAdmin,
		Estado:       dominio.EstadoActiva,
	})
	if errors.Is(err, ErrUsuarioDuplicado) {
		log.Printf("auth-service: otra instancia creó una cuenta con ese sujeto o email al mismo tiempo; no se crea el administrador semilla")
		return nil
	}
	if err != nil {
		return fmt.Errorf("crear administrador: %w", err)
	}
	log.Printf("auth-service: administrador semilla creado (%s)", email)
	return nil
}

// publicar envía el evento después del cambio en la BD. Si falla, solo queda en el
// log y no se revierte (misma regla aceptada en el Reto 4: Outbox en retos posteriores).
func (s *AuthService) publicar(ctx context.Context, tipo string, datos any) {
	sobre, err := eventos.NuevoSobre(tipo, datos, s.ahora())
	if err == nil {
		err = s.publicador.Publicar(ctx, sobre)
	}
	if err != nil {
		log.Printf("auth-service: no se pudo publicar %s: %v", tipo, err)
		return
	}
	log.Printf("auth-service: evento publicado %s (id %s)", tipo, sobre.ID)
}
