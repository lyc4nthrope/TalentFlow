package postgres

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/aplicacion"
)

func TestClasificarErroresDePostgres(t *testing.T) {
	casos := []struct {
		err                   error
		duplicado, permanente bool
	}{
		{&pgconn.PgError{Code: "23505"}, true, false},  // unique_violation
		{&pgconn.PgError{Code: "23514"}, false, true},  // check_violation
		{&pgconn.PgError{Code: "23502"}, false, true},  // not_null_violation
		{&pgconn.PgError{Code: "22001"}, false, true},  // string_data_right_truncation
		{&pgconn.PgError{Code: "22P02"}, false, true},  // invalid_text_representation
		{&pgconn.PgError{Code: "08006"}, false, false}, // connection_failure: transitorio
		{&pgconn.PgError{Code: "57P01"}, false, false}, // admin_shutdown: transitorio
		{errors.New("dial tcp: connection refused"), false, false},
		{fmt.Errorf("envuelto: %w", &pgconn.PgError{Code: "22001"}), false, true},
	}
	for _, c := range casos {
		got := clasificar(c.err, "op")
		if errors.Is(got, aplicacion.ErrUsuarioDuplicado) != c.duplicado || errors.Is(got, aplicacion.ErrRechazoPermanente) != c.permanente {
			t.Errorf("%v → %v", c.err, got)
		}
	}
	if clasificar(nil, "op") != nil {
		t.Error("nil debe seguir siendo nil")
	}
}
