package jwt

import (
	"testing"
)

func TestGenerarTokenIncluyeEmpleadoId(t *testing.T) {
	token, err := NuevoTokenProvider("secret", "auth-service", 24).GenerarToken("user-123", "ana@empresa.com", []string{"USER"}, "E001")
	if err != nil {
		t.Fatalf("GenerarToken devolvió error: %v", err)
	}

	claims, err := NuevoTokenProvider("secret", "auth-service", 24).ValidarToken(token)
	if err != nil {
		t.Fatalf("ValidarToken devolvió error: %v", err)
	}

	if claims.UsuarioID != "user-123" {
		t.Fatalf("UsuarioID esperado user-123, obtenido %q", claims.UsuarioID)
	}

	if claims.EmpleadoID != "E001" {
		t.Fatalf("EmpleadoID esperado E001, obtenido %q", claims.EmpleadoID)
	}
}
