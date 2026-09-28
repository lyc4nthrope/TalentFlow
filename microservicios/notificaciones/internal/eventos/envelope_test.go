package eventos

import (
	"errors"
	"strings"
	"testing"
)

const sobreValido = `{"id":"3f9b2c10-8e4a-4d6b-9f21-7c0a5d8e1234","type":"empleado.creado","version":1,` +
	`"occurredAt":"2026-03-01T14:32:05Z","producer":"empleados-service","data":{"empleadoId":"E001"}}`

func TestParsearSobreValido(t *testing.T) {
	sobre, err := Parsear([]byte(sobreValido))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if sobre.ID != "3f9b2c10-8e4a-4d6b-9f21-7c0a5d8e1234" || sobre.Type != "empleado.creado" || sobre.Version != 1 {
		t.Fatalf("sobre inesperado: %+v", sobre)
	}
}

func TestParsearRechazaMensajesInvalidos(t *testing.T) {
	casos := map[string]string{
		"JSON corrupto":  `{no es json`,
		"sin id":         strings.Replace(sobreValido, `"id":"3f9b2c10-8e4a-4d6b-9f21-7c0a5d8e1234",`, "", 1),
		"sin data":       strings.Replace(sobreValido, `,"data":{"empleadoId":"E001"}`, "", 1),
		"data null":      strings.Replace(sobreValido, `{"empleadoId":"E001"}`, "null", 1),
		"sin occurredAt": strings.Replace(sobreValido, `"occurredAt":"2026-03-01T14:32:05Z",`, "", 1),
		"id demasiado largo": strings.Replace(sobreValido, "3f9b2c10-8e4a-4d6b-9f21-7c0a5d8e1234",
			strings.Repeat("x", LongitudMaximaID+1), 1),
	}
	for nombre, cuerpo := range casos {
		t.Run(nombre, func(t *testing.T) {
			if _, err := Parsear([]byte(cuerpo)); !errors.Is(err, ErrMensajeInvalido) {
				t.Fatalf("se esperaba ErrMensajeInvalido, se obtuvo %v", err)
			}
		})
	}
}
