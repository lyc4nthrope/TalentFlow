package dominio

import (
	"errors"
	"strings"
	"testing"
)

func TestPoliticaDePassword(t *testing.T) {
	casos := []struct {
		password string
		valida   bool
	}{
		{"Clave2026", true},
		{"Aa345678", true},                       // exactamente 8
		{"Aa34567", false},                       // 7
		{"Aa1" + strings.Repeat("x", 69), true},  // exactamente 72 bytes
		{"Aa1" + strings.Repeat("x", 70), false}, // 73 bytes: bcrypt la truncaría
		{"Ññ1" + strings.Repeat("x", 67), true},  // 72 bytes ("Ññ" ocupa 4 bytes)
		{"Ññ1" + strings.Repeat("x", 68), false}, // 73 bytes aunque sean 71 caracteres
		{"clave2026", false},                     // sin mayúscula
		{"CLAVE2026", false},                     // sin minúscula
		{"ClaveSinNum", false},                   // sin dígito
		{"  Clave1  ", true},                     // los espacios cuentan y no se recortan
		{"", false},
	}
	for _, c := range casos {
		err := ValidarPassword(c.password)
		if c.valida && err != nil {
			t.Errorf("%q (%d bytes) debería ser válida: %v", c.password, len(c.password), err)
		}
		if !c.valida && !errors.Is(err, ErrPoliticaPassword) {
			t.Errorf("%q (%d bytes) debería ser inválida", c.password, len(c.password))
		}
	}
}

func TestHashTokenEsSHA256Hex(t *testing.T) {
	// SHA-256 de "abc" (vector de prueba estándar).
	if got := HashToken("abc"); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("HashToken = %s", got)
	}
	if HashToken("a") == HashToken("b") {
		t.Fatal("hashes iguales para tokens distintos")
	}
}
