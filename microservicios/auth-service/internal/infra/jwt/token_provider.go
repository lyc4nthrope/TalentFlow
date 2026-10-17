// Package jwt emite y valida los tokens de acceso (HS256) del Reto 5.
package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/aplicacion"
)

// Claims del token de acceso: exactamente la estructura del PDF (sub, role, iat,
// exp) más email e iss. La contraseña nunca viaja en el token.
type Claims struct {
	Role  string `json:"role"`
	Email string `json:"email"`
	jwt.RegisteredClaims
}

type TokenProvider struct {
	secreto  []byte
	emisor   string
	duracion time.Duration
	ahora    func() time.Time
}

// NuevoTokenProvider exige un secreto no vacío: firmar con una clave vacía
// permitiría a cualquiera fabricar tokens.
func NuevoTokenProvider(secreto, emisor string, duracion time.Duration, ahora func() time.Time) (*TokenProvider, error) {
	if secreto == "" {
		return nil, errors.New("JWT_SECRET vacío")
	}
	if duracion <= 0 {
		return nil, errors.New("la duración del token de acceso debe ser positiva")
	}
	return &TokenProvider{secreto: []byte(secreto), emisor: emisor, duracion: duracion, ahora: ahora}, nil
}

// Emitir firma un token de acceso para el sujeto (empleadoId o "admin").
func (p *TokenProvider) Emitir(sujeto, rol, email string) (string, error) {
	ahora := p.ahora().UTC().Truncate(time.Second)
	claims := Claims{
		Role:  rol,
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sujeto,
			Issuer:    p.emisor,
			IssuedAt:  jwt.NewNumericDate(ahora),
			ExpiresAt: jwt.NewNumericDate(ahora.Add(p.duracion)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(p.secreto)
}

// Validar verifica firma (solo HS256: rechaza "none" y cualquier otro algoritmo),
// emisor, expiración obligatoria y que el token traiga sujeto y rol.
func (p *TokenProvider) Validar(token string) (*aplicacion.Identidad, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return p.secreto, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(p.emisor),
		jwt.WithTimeFunc(p.ahora),
	)
	if err != nil {
		return nil, fmt.Errorf("token de acceso inválido: %w", err)
	}
	if claims.Subject == "" || claims.Role == "" {
		return nil, errors.New("token de acceso sin sub o role")
	}
	return &aplicacion.Identidad{Sujeto: claims.Subject, Rol: claims.Role, Email: claims.Email}, nil
}
