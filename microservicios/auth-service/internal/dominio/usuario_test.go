package dominio

import "testing"

// Solo una cuenta ACTIVA y con contraseña puede iniciar sesión: ni una pendiente de
// activación, ni una suspendida por vacaciones, ni una retirada.
func TestPuedeAutenticarse(t *testing.T) {
	casos := []struct {
		estado, hash string
		puede        bool
	}{
		{EstadoActiva, "hash", true},
		{EstadoActiva, "", false},
		{EstadoPendienteActivacion, "", false},
		{EstadoPendienteActivacion, "hash", false},
		{EstadoSuspendidaTemporal, "hash", false},
		{EstadoDesactivadaPermanente, "hash", false},
	}
	for _, c := range casos {
		u := Usuario{Estado: c.estado, PasswordHash: c.hash}
		if got := u.PuedeAutenticarse(); got != c.puede {
			t.Errorf("estado %s, contraseña %q: PuedeAutenticarse = %v; se esperaba %v", c.estado, c.hash, got, c.puede)
		}
	}
}

// En la BD solo se guarda el hash: es el SHA-256 en hexadecimal y no contiene el token.
func TestHashTokenEsSHA256Hexadecimal(t *testing.T) {
	const esperado = "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae" // sha256("foo")
	if got := HashToken("foo"); got != esperado {
		t.Fatalf("HashToken = %s", got)
	}
}
