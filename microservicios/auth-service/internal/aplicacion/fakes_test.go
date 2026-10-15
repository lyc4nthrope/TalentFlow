package aplicacion

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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
}

func nuevoRepo() *repoMemoria { return &repoMemoria{usuarios: map[string]*dominio.Usuario{}} }

func (r *repoMemoria) copia(u *dominio.Usuario) *dominio.Usuario {
	if u == nil {
		return nil
	}
	c := *u
	return &c
}

func (r *repoMemoria) buscar(pred func(*dominio.Usuario) bool) (*dominio.Usuario, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fallar != nil {
		return nil, r.fallar
	}
	for _, u := range r.usuarios {
		if pred(u) {
			return r.copia(u), nil
		}
	}
	return nil, nil
}

func (r *repoMemoria) BuscarPorEmail(_ context.Context, email string) (*dominio.Usuario, error) {
	return r.buscar(func(u *dominio.Usuario) bool { return u.Email == strings.ToLower(email) })
}
func (r *repoMemoria) BuscarPorEmpleadoID(_ context.Context, id string) (*dominio.Usuario, error) {
	return r.buscar(func(u *dominio.Usuario) bool { return u.EmpleadoID != "" && u.EmpleadoID == id })
}
func (r *repoMemoria) BuscarPorSujeto(_ context.Context, s string) (*dominio.Usuario, error) {
	return r.buscar(func(u *dominio.Usuario) bool { return u.Sujeto == s })
}
func (r *repoMemoria) BuscarPorTokenHash(_ context.Context, h string) (*dominio.Usuario, error) {
	return r.buscar(func(u *dominio.Usuario) bool { return u.TokenHash != "" && u.TokenHash == h })
}

func (r *repoMemoria) Crear(_ context.Context, u *dominio.Usuario) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fallar != nil {
		return r.fallar
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
	u, ok := r.usuarios[id]
	if !ok {
		return errors.New("no existe")
	}
	u.PasswordHash = pass
	u.TokenHash, u.TokenTipo, u.TokenExpiraEn = "", "", time.Time{}
	return nil
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
