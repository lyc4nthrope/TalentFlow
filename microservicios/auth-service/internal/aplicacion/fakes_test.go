package aplicacion

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/eventos"
)

// repoMemoria imita al repositorio de Postgres, incluidas sus restricciones únicas y
// el UPDATE condicional de ConsumirToken.
type repoMemoria struct {
	mu        sync.Mutex
	usuarios  map[string]*dominio.Usuario
	siguiente int
	fallar    error // si no es nil, toda operación falla con este error
	// consumoGanadoPorOtro simula que otro uso simultáneo consumió el token primero.
	consumoGanadoPorOtro bool
	// procesados imita la tabla eventos_procesados (deduplicación, D4).
	procesados map[string]bool
	// transaccion serializa ProcesarUnaVez completo (comprobar, aplicar y marcar),
	// como lo hace la BD con el INSERT del id: dos entregas simultáneas del mismo
	// mensaje no pueden aplicarse ambas.
	transaccion sync.Mutex
	// fallarEfecto hace fallar el siguiente UPDATE del ciclo de vida (prueba de rollback).
	fallarEfecto error
	// fallarEn hace fallar solo la operación con ese nombre (p. ej. "BuscarPorEmail"),
	// para llegar a un fallo que ocurre después de otras operaciones que sí funcionan.
	fallarEn map[string]error
	// competidor, si no es nil, se inserta justo antes del próximo Crear: simula otra
	// transacción que crea una cuenta que choca y confirma primero (carrera).
	competidor *dominio.Usuario
}

// fallo devuelve el error configurado para la operación (nil si debe funcionar).
func (r *repoMemoria) fallo(op string) error {
	if r.fallar != nil {
		return r.fallar
	}
	return r.fallarEn[op]
}

func nuevoRepo() *repoMemoria {
	return &repoMemoria{usuarios: map[string]*dominio.Usuario{}, procesados: map[string]bool{}}
}

func (r *repoMemoria) copia(u *dominio.Usuario) *dominio.Usuario {
	if u == nil {
		return nil
	}
	c := *u
	return &c
}

func (r *repoMemoria) buscar(op string, pred func(*dominio.Usuario) bool) (*dominio.Usuario, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.fallo(op); err != nil {
		return nil, err
	}
	for _, u := range r.usuarios {
		if pred(u) {
			return r.copia(u), nil
		}
	}
	return nil, nil
}

func (r *repoMemoria) BuscarPorEmail(_ context.Context, email string) (*dominio.Usuario, error) {
	return r.buscar("BuscarPorEmail", func(u *dominio.Usuario) bool { return u.Email == strings.ToLower(email) })
}
func (r *repoMemoria) BuscarPorEmpleadoID(_ context.Context, id string) (*dominio.Usuario, error) {
	return r.buscar("BuscarPorEmpleadoID", func(u *dominio.Usuario) bool { return u.EmpleadoID != "" && u.EmpleadoID == id })
}
func (r *repoMemoria) BuscarPorSujeto(_ context.Context, s string) (*dominio.Usuario, error) {
	return r.buscar("BuscarPorSujeto", func(u *dominio.Usuario) bool { return u.Sujeto == s })
}
func (r *repoMemoria) BuscarPorTokenHash(_ context.Context, h string) (*dominio.Usuario, error) {
	return r.buscar("BuscarPorTokenHash", func(u *dominio.Usuario) bool { return u.TokenHash != "" && u.TokenHash == h })
}

func (r *repoMemoria) Crear(_ context.Context, u *dominio.Usuario) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.fallo("Crear"); err != nil {
		return err
	}
	if c := r.competidor; c != nil {
		r.competidor = nil
		r.siguiente++
		c.ID = fmt.Sprintf("u%d", r.siguiente)
		r.usuarios[c.ID] = c
	}
	for _, e := range r.usuarios {
		if e.Email == strings.ToLower(u.Email) || e.Sujeto == u.Sujeto || (u.EmpleadoID != "" && e.EmpleadoID == u.EmpleadoID) {
			return ErrUsuarioDuplicado
		}
	}
	r.siguiente++
	u.ID = fmt.Sprintf("u%d", r.siguiente)
	c := *u
	c.Email = strings.ToLower(c.Email)
	r.usuarios[u.ID] = &c
	return nil
}

func (r *repoMemoria) GuardarToken(_ context.Context, id, hash string, tipo dominio.TipoToken, expira time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.fallo("GuardarToken"); err != nil {
		return err
	}
	u, ok := r.usuarios[id]
	if !ok {
		return errors.New("no existe")
	}
	u.TokenHash, u.TokenTipo, u.TokenExpiraEn = hash, tipo, expira
	return nil
}

func (r *repoMemoria) ConsumirToken(_ context.Context, id, hash, estadoEsperado, pass, estado string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.fallo("ConsumirToken"); err != nil {
		return false, err
	}
	u, ok := r.usuarios[id]
	if !ok || u.TokenHash != hash || u.Estado != estadoEsperado || r.consumoGanadoPorOtro {
		return false, nil
	}
	u.PasswordHash, u.Estado = pass, estado
	u.TokenHash, u.TokenTipo, u.TokenExpiraEn = "", "", time.Time{}
	return true, nil
}

func (r *repoMemoria) ActualizarPassword(_ context.Context, id, pass string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.fallo("ActualizarPassword"); err != nil {
		return err
	}
	u, ok := r.usuarios[id]
	if !ok {
		return errors.New("no existe")
	}
	u.PasswordHash = pass
	u.TokenHash, u.TokenTipo, u.TokenExpiraEn = "", "", time.Time{}
	return nil
}

// ProcesarUnaVez imita la transacción: registra el id y aplica fn; si fn falla
// restaura las cuentas y no registra el id (rollback).
func (r *repoMemoria) ProcesarUnaVez(_ context.Context, id string, fn func(Repositorio) error) (bool, error) {
	r.transaccion.Lock()
	defer r.transaccion.Unlock()
	r.mu.Lock()
	if r.procesados[id] {
		r.mu.Unlock()
		return true, nil
	}
	foto := map[string]dominio.Usuario{}
	for k, u := range r.usuarios {
		foto[k] = *u
	}
	r.mu.Unlock()

	err := fn(r)

	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		r.usuarios = map[string]*dominio.Usuario{}
		for k, u := range foto {
			c := u
			r.usuarios[k] = &c
		}
		return false, err
	}
	r.procesados[id] = true
	return false, nil
}

// actualizar aplica cambio a la cuenta si cumple la condición (UPDATE condicional).
func (r *repoMemoria) actualizar(id string, condicion func(*dominio.Usuario) bool, cambio func(*dominio.Usuario)) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fallarEfecto != nil {
		err := r.fallarEfecto
		r.fallarEfecto = nil
		return false, err
	}
	u, ok := r.usuarios[id]
	if !ok || !condicion(u) {
		return false, nil
	}
	cambio(u)
	return true, nil
}

func (r *repoMemoria) Retirar(_ context.Context, id string) (bool, error) {
	return r.actualizar(id,
		func(u *dominio.Usuario) bool { return u.Estado != dominio.EstadoDesactivadaPermanente },
		func(u *dominio.Usuario) {
			u.Estado, u.VacacionesSuspension = dominio.EstadoDesactivadaPermanente, ""
			u.TokenHash, u.TokenTipo, u.TokenExpiraEn = "", "", time.Time{}
		})
}

func (r *repoMemoria) Suspender(_ context.Context, id, vac string) (bool, error) {
	return r.actualizar(id,
		func(u *dominio.Usuario) bool { return u.Estado == dominio.EstadoActiva },
		func(u *dominio.Usuario) { u.Estado, u.VacacionesSuspension = dominio.EstadoSuspendidaTemporal, vac })
}

func (r *repoMemoria) CambiarSuspension(_ context.Context, id, vac string) (bool, error) {
	return r.actualizar(id,
		func(u *dominio.Usuario) bool { return u.Estado == dominio.EstadoSuspendidaTemporal },
		func(u *dominio.Usuario) { u.VacacionesSuspension = vac })
}

func (r *repoMemoria) Reactivar(_ context.Context, id, vac string) (bool, error) {
	return r.actualizar(id,
		func(u *dominio.Usuario) bool {
			return u.Estado == dominio.EstadoSuspendidaTemporal && u.VacacionesSuspension == vac
		},
		func(u *dominio.Usuario) { u.Estado, u.VacacionesSuspension = dominio.EstadoActiva, "" })
}

// cuenta devuelve una copia de la cuenta con ese sujeto (nil si no existe).
func (r *repoMemoria) cuenta(sujeto string) *dominio.Usuario {
	u, _ := r.BuscarPorSujeto(context.Background(), sujeto)
	return u
}

// estado cambia el estado de una cuenta directamente (simula la F3: vacaciones o retiro).
func (r *repoMemoria) fijarEstado(sujeto, estado string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.usuarios {
		if u.Sujeto == sujeto {
			u.Estado = estado
		}
	}
}

type publicadorEspia struct {
	mu     sync.Mutex
	sobres []eventos.Envelope
	fallar error
}

func (p *publicadorEspia) Publicar(_ context.Context, s eventos.Envelope) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fallar != nil {
		return p.fallar
	}
	p.sobres = append(p.sobres, s)
	return nil
}

func (p *publicadorEspia) tipos() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var t []string
	for _, s := range p.sobres {
		t = append(t, s.Type)
	}
	return t
}

func (p *publicadorEspia) ultimo() eventos.Envelope {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sobres[len(p.sobres)-1]
}

// emisorQueFalla simula un fallo al firmar el token de acceso.
type emisorQueFalla struct{ emisorFalso }

func (emisorQueFalla) Emitir(string, string, string) (string, error) {
	return "", errors.New("firma no disponible")
}

// generadorQueFalla simula que no se pudo generar un token aleatorio.
func generadorQueFalla() (string, error) { return "", errors.New("sin entropía") }

// emisorFalso emite "token:<sujeto>:<rol>" y solo valida lo que emitió.
type emisorFalso struct{}

func (emisorFalso) Emitir(sujeto, rol, email string) (string, error) {
	return "token:" + sujeto + ":" + rol + ":" + email, nil
}

func (emisorFalso) Validar(token string) (*Identidad, error) {
	partes := strings.Split(token, ":")
	if len(partes) != 4 || partes[0] != "token" {
		return nil, errors.New("inválido")
	}
	return &Identidad{Sujeto: partes[1], Rol: partes[2], Email: partes[3]}, nil
}

// generadorSecuencial devuelve tok-1, tok-2, ... para poder predecir los tokens.
func generadorSecuencial() GeneradorTokens {
	n := 0
	return func() (string, error) {
		n++
		return fmt.Sprintf("tok-%d", n), nil
	}
}

// relojManual permite avanzar el tiempo en las pruebas.
type relojManual struct{ t time.Time }

func (r *relojManual) ahora() time.Time        { return r.t }
func (r *relojManual) avanzar(d time.Duration) { r.t = r.t.Add(d) }

var contadorEventos atomic.Int64

// nuevoEventoID devuelve un id de evento distinto en cada llamada.
func nuevoEventoID() string { return fmt.Sprintf("evt-%d", contadorEventos.Add(1)) }
