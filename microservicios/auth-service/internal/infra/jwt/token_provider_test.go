package jwt

import (
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var instante = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

func proveedor(t *testing.T, secreto string, ahora time.Time) *TokenProvider {
	t.Helper()
	p, err := NuevoTokenProvider(secreto, "auth-service", time.Hour, func() time.Time { return ahora })
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func payload(t *testing.T, token string) map[string]any {
	t.Helper()
	partes := strings.Split(token, ".")
	crudo, err := base64.RawURLEncoding.DecodeString(partes[1])
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(crudo, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// Los claims del PDF (sub, role, iat, exp) más email e iss, y nada más.
func TestElTokenTraeLosClaimsDelPDFMasEmailEIss(t *testing.T) {
	token, err := proveedor(t, "secreto", instante).Emitir("E001", "USER", "juan@empresa.com")
	if err != nil {
		t.Fatal(err)
	}
	m := payload(t, token)
	var claves []string
	for k := range m {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	if got, want := strings.Join(claves, ","), "email,exp,iat,iss,role,sub"; got != want {
		t.Fatalf("claims = %s; se esperaba %s", got, want)
	}
	if m["sub"] != "E001" || m["role"] != "USER" {
		t.Fatalf("sub/role = %v/%v", m["sub"], m["role"])
	}
	if exp, iat := m["exp"].(float64), m["iat"].(float64); exp-iat != 3600 {
		t.Fatalf("exp - iat = %v; se esperaba 3600", exp-iat)
	}
}

func TestValidarDevuelveLaIdentidad(t *testing.T) {
	p := proveedor(t, "secreto", instante)
	token, _ := p.Emitir("admin", "ADMIN", "admin@empresa.com")
	id, err := p.Validar(token)
	if err != nil {
		t.Fatal(err)
	}
	if id.Sujeto != "admin" || id.Rol != "ADMIN" || id.Email != "admin@empresa.com" {
		t.Fatalf("identidad = %+v", id)
	}
}

func TestRechazaTokenFirmadoConOtroSecreto(t *testing.T) {
	token, _ := proveedor(t, "otro-secreto", instante).Emitir("E001", "ADMIN", "x@y.com")
	if _, err := proveedor(t, "secreto", instante).Validar(token); err == nil {
		t.Fatal("aceptó un token firmado con otra clave")
	}
}

func TestRechazaTokenAlterado(t *testing.T) {
	p := proveedor(t, "secreto", instante)
	token, _ := p.Emitir("E001", "USER", "x@y.com")
	partes := strings.Split(token, ".")
	falso, _ := json.Marshal(map[string]any{"sub": "E001", "role": "ADMIN", "email": "x@y.com",
		"iat": instante.Unix(), "exp": instante.Add(time.Hour).Unix()})
	partes[1] = base64.RawURLEncoding.EncodeToString(falso)
	if _, err := p.Validar(strings.Join(partes, ".")); err == nil {
		t.Fatal("aceptó un token con el payload alterado (escalada a ADMIN)")
	}
}

func TestRechazaTokenExpirado(t *testing.T) {
	token, _ := proveedor(t, "secreto", instante).Emitir("E001", "USER", "x@y.com")
	if _, err := proveedor(t, "secreto", instante.Add(61*time.Minute)).Validar(token); err == nil {
		t.Fatal("aceptó un token expirado")
	}
}

func TestRechazaAlgoritmoNone(t *testing.T) {
	claims := Claims{Role: "ADMIN", RegisteredClaims: jwt.RegisteredClaims{
		Subject: "admin", ExpiresAt: jwt.NewNumericDate(instante.Add(time.Hour))}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := proveedor(t, "secreto", instante).Validar(token); err == nil {
		t.Fatal("aceptó un token sin firma (alg none)")
	}
}

func TestRechazaTokenSinExpiracion(t *testing.T) {
	claims := Claims{Role: "USER", RegisteredClaims: jwt.RegisteredClaims{Subject: "E001"}}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secreto"))
	if _, err := proveedor(t, "secreto", instante).Validar(token); err == nil {
		t.Fatal("aceptó un token sin exp")
	}
}

func TestRechazaOtrosAlgoritmosHMACConElMismoSecreto(t *testing.T) {
	// Prueba que el algoritmo está fijado en HS256: la firma sería válida con la clave.
	for _, metodo := range []jwt.SigningMethod{jwt.SigningMethodHS384, jwt.SigningMethodHS512} {
		claims := Claims{Role: "ADMIN", RegisteredClaims: jwt.RegisteredClaims{Subject: "admin", Issuer: "auth-service",
			ExpiresAt: jwt.NewNumericDate(instante.Add(time.Hour))}}
		token, _ := jwt.NewWithClaims(metodo, claims).SignedString([]byte("secreto"))
		if _, err := proveedor(t, "secreto", instante).Validar(token); err == nil {
			t.Fatalf("aceptó un token %s", metodo.Alg())
		}
	}
}

func TestRechazaTokenDeOtroEmisor(t *testing.T) {
	otro, _ := NuevoTokenProvider("secreto", "otro-servicio", time.Hour, func() time.Time { return instante })
	token, _ := otro.Emitir("E001", "USER", "x@y.com")
	if _, err := proveedor(t, "secreto", instante).Validar(token); err == nil {
		t.Fatal("aceptó un token de otro emisor")
	}
}

func TestNoSeCreaConSecretoVacio(t *testing.T) {
	if _, err := NuevoTokenProvider("", "auth-service", time.Hour, time.Now); err == nil {
		t.Fatal("se creó un proveedor con secreto vacío")
	}
}

func TestNoSeCreaConDuracionCeroONegativa(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Second} {
		if p, err := NuevoTokenProvider("secreto", "auth-service", d, time.Now); err == nil || p != nil {
			t.Fatalf("duración %v: se creó un proveedor que emitiría tokens ya vencidos", d)
		}
	}
}

// Un token bien firmado por este emisor pero sin sub o sin role no identifica a nadie
// (o no permite autorizar): se rechaza aunque la firma sea válida.
func TestRechazaTokenSinSujetoOSinRol(t *testing.T) {
	casos := map[string]Claims{
		"sin sub":  {Role: "USER"},
		"sin role": {RegisteredClaims: jwt.RegisteredClaims{Subject: "E001"}},
	}
	for nombre, claims := range casos {
		t.Run(nombre, func(t *testing.T) {
			claims.Issuer = "auth-service"
			claims.ExpiresAt = jwt.NewNumericDate(instante.Add(time.Hour))
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secreto"))
			if err != nil {
				t.Fatal(err)
			}
			if id, err := proveedor(t, "secreto", instante).Validar(token); err == nil || id != nil {
				t.Fatalf("aceptó un token %s: %+v", nombre, id)
			}
		})
	}
}
