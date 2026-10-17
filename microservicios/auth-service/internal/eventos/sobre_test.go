package eventos

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNuevoSobreTieneElEnvelopeDelCatalogo(t *testing.T) {
	ahora := time.Date(2026, 10, 15, 7, 30, 15, 999, time.FixedZone("COT", -5*3600))
	s, err := NuevoSobre("usuario.creado", map[string]string{"email": "a@b.com"}, ahora)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(s.ID); err != nil {
		t.Fatalf("id no es UUID: %q", s.ID)
	}
	if s.Type != "usuario.creado" || s.Version != 1 || s.Producer != "auth-service" || s.OccurredAt != "2026-10-15T12:30:15Z" {
		t.Fatalf("sobre = %+v", s)
	}
	crudo, _ := json.Marshal(s)
	var m map[string]any
	_ = json.Unmarshal(crudo, &m)
	var claves []string
	for k := range m {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	if got := strings.Join(claves, ","); got != "data,id,occurredAt,producer,type,version" {
		t.Fatalf("campos = %s", got)
	}
	otro, _ := NuevoSobre("usuario.creado", nil, ahora)
	if otro.ID == s.ID {
		t.Fatal("dos eventos con el mismo id")
	}
}
