package aplicacion

import (
	"context"
	"fmt"
	"strings"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
)

// Ciclo de vida de la cuenta dirigido por eventos (Reto 5, F3: D4, D5, D18, D19).
// Cada caso de uso registra el id del evento y aplica su efecto en una sola
// transacción, y publica DESPUÉS del commit y solo si el estado cambió.

// Límites de las columnas (schema.sql): eventos_procesados.id y vacaciones_suspension.
const (
	longitudMaximaEventoID     = 100
	longitudMaximaVacacionesID = 20
)

func validarEventoID(id string) error {
	if strings.TrimSpace(id) == "" || len(id) > longitudMaximaEventoID {
		return fmt.Errorf("%w: id del evento vacío o de más de %d caracteres", ErrSolicitudInvalida, longitudMaximaEventoID)
	}
	return nil
}

func validarEmpleadoID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > longitudMaximaEmpleadoID {
		return "", fmt.Errorf("%w: empleadoId vacío o de más de %d caracteres", ErrSolicitudInvalida, longitudMaximaEmpleadoID)
	}
	return id, nil
}

func validarVacacionesID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > longitudMaximaVacacionesID {
		return "", fmt.Errorf("%w: vacacionesId vacío o de más de %d caracteres", ErrSolicitudInvalida, longitudMaximaVacacionesID)
	}
	return id, nil
}

// efectoDelCiclo busca la cuenta del empleado dentro de la transacción y aplica
// aplicar. Devuelve el evento a publicar tras el commit (nil si no hubo transición).
type efectoDelCiclo func(ctx context.Context, repo Repositorio, u *dominio.Usuario) (tipo string, datos any, err error)

func (s *AuthService) procesarCiclo(ctx context.Context, tipoEvento, eventoID, empleadoID string, aplicar efectoDelCiclo) error {
	if err := validarEventoID(eventoID); err != nil {
		return err
	}
	var tipo string
	var datos any
	duplicado, err := s.repo.ProcesarUnaVez(ctx, eventoID, func(repo Repositorio) error {
		// Se busca por empleado_id (no por sujeto): un evento nunca puede tocar al
		// administrador semilla, que no es un empleado.
		u, err := repo.BuscarPorEmpleadoID(ctx, empleadoID)
		if err != nil {
			return fmt.Errorf("buscar cuenta del empleado: %w", err)
		}
		if u == nil {
			registrar(ctx, "%s para el empleado %s sin cuenta; no hay nada que cambiar", tipoEvento, empleadoID)
			return nil
		}
		tipo, datos, err = aplicar(ctx, repo, u)
		return err
	})
	if duplicado {
		registrar(ctx, "evento duplicado descartado: %s ya se había procesado (%s)", eventoID, tipoEvento)
		return nil
	}
	if err != nil {
		return err
	}
	if tipo != "" {
		s.publicar(ctx, tipo, datos)
	}
	return nil
}

// RetirarCuenta reacciona a empleado.retirado: la cuenta pasa a DESACTIVADA_PERMANENTE
// desde cualquier estado, se borra el token pendiente (un enlace de activación o
// recuperación ya no sirve) y se publica cuenta.desactivada con motivo RETIRO.
func (s *AuthService) RetirarCuenta(ctx context.Context, eventoID string, e dominio.EmpleadoRetirado) error {
	empleadoID, err := validarEmpleadoID(e.EmpleadoID)
	if err != nil {
		return err
	}
	return s.procesarCiclo(ctx, dominio.EventoEmpleadoRetirado, eventoID, empleadoID,
		func(ctx context.Context, repo Repositorio, u *dominio.Usuario) (string, any, error) {
			cambio, err := repo.Retirar(ctx, u.ID)
			if err != nil || !cambio {
				if err == nil {
					registrar(ctx, "la cuenta del empleado %s ya estaba retirada", empleadoID)
				}
				return "", nil, err
			}
			registrar(ctx, "cuenta del empleado %s desactivada de forma permanente (retiro)", empleadoID)
			return dominio.EventoCuentaDesactivada, dominio.CuentaDesactivada{
				EmpleadoID: u.EmpleadoID, Email: u.Email, Motivo: dominio.MotivoRetiro, Permanente: true,
			}, nil
		})
}

// SuspenderPorVacaciones reacciona a vacaciones.iniciadas. Una cuenta ACTIVA pasa a
// SUSPENDIDA_TEMPORAL recordando el id de esas vacaciones (D18) y se publica
// cuenta.desactivada con motivo VACACIONES. Si ya estaba suspendida solo se
// actualiza el id (períodos contiguos) sin publicar. Una cuenta pendiente de
// activación o retirada no cambia.
func (s *AuthService) SuspenderPorVacaciones(ctx context.Context, eventoID string, e dominio.VacacionesIniciadas) error {
	empleadoID, err := validarEmpleadoID(e.EmpleadoID)
	if err != nil {
		return err
	}
	vacacionesID, err := validarVacacionesID(e.VacacionesID)
	if err != nil {
		return err
	}
	return s.procesarCiclo(ctx, dominio.EventoVacacionesIniciadas, eventoID, empleadoID,
		func(ctx context.Context, repo Repositorio, u *dominio.Usuario) (string, any, error) {
			switch u.Estado {
			case dominio.EstadoActiva:
				cambio, err := repo.Suspender(ctx, u.ID, vacacionesID)
				if err != nil || !cambio {
					return "", nil, err
				}
				registrar(ctx, "cuenta del empleado %s suspendida por las vacaciones %s", empleadoID, vacacionesID)
				return dominio.EventoCuentaDesactivada, dominio.CuentaDesactivada{
					EmpleadoID: u.EmpleadoID, Email: u.Email, Motivo: dominio.MotivoVacaciones, Permanente: false,
				}, nil
			case dominio.EstadoSuspendidaTemporal:
				_, err := repo.CambiarSuspension(ctx, u.ID, vacacionesID)
				if err == nil {
					registrar(ctx, "la cuenta del empleado %s ya estaba suspendida; ahora la suspensión es de las vacaciones %s",
						empleadoID, vacacionesID)
				}
				return "", nil, err
			default:
				registrar(ctx, "vacaciones %s sin efecto: la cuenta del empleado %s está %s",
					vacacionesID, empleadoID, u.Estado)
				return "", nil, nil
			}
		})
}

// ReactivarPorFinDeVacaciones reacciona a vacaciones.finalizadas. Solo reactiva una
// cuenta SUSPENDIDA_TEMPORAL por esas mismas vacaciones (D18) y publica
// cuenta.activada con motivo FIN_VACACIONES. Caso borde (D5): una cuenta retirada
// durante sus vacaciones nunca se reactiva.
func (s *AuthService) ReactivarPorFinDeVacaciones(ctx context.Context, eventoID string, e dominio.VacacionesFinalizadas) error {
	empleadoID, err := validarEmpleadoID(e.EmpleadoID)
	if err != nil {
		return err
	}
	vacacionesID, err := validarVacacionesID(e.VacacionesID)
	if err != nil {
		return err
	}
	return s.procesarCiclo(ctx, dominio.EventoVacacionesFinalizadas, eventoID, empleadoID,
		func(ctx context.Context, repo Repositorio, u *dominio.Usuario) (string, any, error) {
			if u.Estado == dominio.EstadoDesactivadaPermanente {
				registrar(ctx, "cuenta retirada: no se reactiva (empleado %s, vacaciones %s)", empleadoID, vacacionesID)
				return "", nil, nil
			}
			cambio, err := repo.Reactivar(ctx, u.ID, vacacionesID)
			if err != nil {
				return "", nil, err
			}
			if !cambio {
				registrar(ctx, "fin de las vacaciones %s sin efecto: la cuenta del empleado %s está %s (suspensión de %q)",
					vacacionesID, empleadoID, u.Estado, u.VacacionesSuspension)
				return "", nil, nil
			}
			registrar(ctx, "cuenta del empleado %s reactivada al terminar las vacaciones %s", empleadoID, vacacionesID)
			return dominio.EventoCuentaActivada, dominio.CuentaActivada{
				EmpleadoID: u.EmpleadoID, Email: u.Email, Motivo: dominio.MotivoFinVacaciones,
			}, nil
		})
}
