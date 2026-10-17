package postgres

// Concurrencia contra PostgreSQL real (ver repositorio_test.go para ejecutarlas).

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/aplicacion"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/eventos"
)

type publicadorNulo struct{}

func (publicadorNulo) Publicar(context.Context, eventos.Envelope) error { return nil }

type emisorNulo struct{}

func (emisorNulo) Emitir(string, string, string) (string, error) { return "", nil }
func (emisorNulo) Validar(string) (*aplicacion.Identidad, error) {
	return nil, errors.New("sin uso")
}

func servicioReal(t *testing.T, r *Repositorio) *aplicacion.AuthService {
	t.Helper()
	svc, err := aplicacion.NuevoAuthService(r, publicadorNulo{}, emisorNulo{},
		func() (string, error) { return "tok", nil }, time.Now, aplicacion.Opciones{CostoBcrypt: bcrypt.MinCost})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// D18 con manejadores concurrentes: mientras vacaciones.finalizadas(A) reactiva la
// cuenta sin confirmar todavía, llega vacaciones.iniciadas(B). Con la fila bloqueada
// (SELECT ... FOR UPDATE), B espera, lee la cuenta ya ACTIVA y la suspende. Sin el
// bloqueo, B decidiría con el estado viejo (SUSPENDIDA por A), solo intentaría
// cambiar el id, y la cuenta quedaría ACTIVA durante todas las vacaciones B.
func TestManejadoresConcurrentesDeLaMismaCuentaSeSerializan(t *testing.T) {
	r := repoReal(t)
	svc := servicioReal(t, r)
	u := activa(t, r, "E001")
	if _, err := r.Suspender(bg, u.ID, "V-A"); err != nil {
		t.Fatal(err)
	}

	reactivada, liberar := make(chan struct{}), make(chan struct{})
	finA := make(chan error, 1)
	go func() {
		_, err := r.ProcesarUnaVez(bg, "fA", func(repo aplicacion.Repositorio) error {
			if _, err := repo.Reactivar(bg, u.ID, "V-A"); err != nil {
				return err
			}
			close(reactivada)
			<-liberar // mantiene la transacción abierta, con la fila bloqueada
			return nil
		})
		finA <- err
	}()
	<-reactivada

	inicioB := make(chan error, 1)
	go func() {
		inicioB <- svc.SuspenderPorVacaciones(bg, "iB", dominio.VacacionesIniciadas{VacacionesID: "V-B", EmpleadoID: "E001"})
	}()
	time.Sleep(300 * time.Millisecond) // B llega mientras A sigue sin confirmar
	close(liberar)

	if err := <-finA; err != nil {
		t.Fatal(err)
	}
	if err := <-inicioB; err != nil {
		t.Fatal(err)
	}
	if got := leer(t, r, "E001"); got.Estado != dominio.EstadoSuspendidaTemporal || got.VacacionesSuspension != "V-B" {
		t.Fatalf("estado final = %s (%q); se esperaba SUSPENDIDA_TEMPORAL por V-B", got.Estado, got.VacacionesSuspension)
	}
}

// Dos entregas simultáneas del mismo mensaje: el INSERT del id en eventos_procesados
// hace que la segunda espere a la primera y la descarte como duplicada.
func TestElMismoIdProcesadoEnParaleloSeAplicaUnaSolaVez(t *testing.T) {
	r := repoReal(t)
	u := activa(t, r, "E001")
	var efectos atomic.Int32
	var nuevos, duplicados atomic.Int32

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dup, err := r.ProcesarUnaVez(bg, "mismo-id", func(repo aplicacion.Repositorio) error {
				efectos.Add(1)
				time.Sleep(200 * time.Millisecond) // la otra entrega llega mientras tanto
				_, err := repo.Suspender(bg, u.ID, "V-A")
				return err
			})
			if err != nil {
				t.Error(err)
			}
			if dup {
				duplicados.Add(1)
			} else {
				nuevos.Add(1)
			}
		}()
	}
	wg.Wait()
	if efectos.Load() != 1 || nuevos.Load() != 1 || duplicados.Load() != 1 {
		t.Fatalf("efectos=%d nuevos=%d duplicados=%d; se esperaba 1/1/1", efectos.Load(), nuevos.Load(), duplicados.Load())
	}
}
