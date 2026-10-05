package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenClaims struct {
	UsuarioID string `json:"sub"`
	Email     string `json:"email"`
	Rol       string `json:"role"`
	jwt.RegisteredClaims
}

type TokenProvider struct {
	secretKey []byte
	issuer    string
	duracion  time.Duration
}

func NuevoTokenProvider(secretKey string, issuer string, duracionHoras int) *TokenProvider {
	return &TokenProvider{
		secretKey: []byte(secretKey),
		issuer:    issuer,
		duracion:  time.Duration(duracionHoras) * time.Hour,
	}
}

// GenerarToken crea y firma un nuevo JWT para el usuario
func (p *TokenProvider) GenerarToken(usuarioID, email, rol string) (string, error) {
	ahora := time.Now().UTC()
	claims := TokenClaims{
		UsuarioID: usuarioID,
		Email:     email,
		Rol:       rol,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    p.issuer,
			Subject:   usuarioID,
			IssuedAt:  jwt.NewNumericDate(ahora),
			ExpiresAt: jwt.NewNumericDate(ahora.Add(p.duracion)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(p.secretKey)
}

// ValidarToken parsea y verifica la firma del token JWT
func (p *TokenProvider) ValidarToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de firma inesperado: %v", t.Header["alg"])
		}
		return p.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("token inválido o expirado")
	}

	return claims, nil
}