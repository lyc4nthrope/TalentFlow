package dominio

import (
	"errors"
	"unicode"
)

// Límites de la política de contraseñas (Reto 5, D14). bcrypt ignora todo lo que
// pasa de 72 bytes, así que una contraseña más larga daría una falsa sensación de
// seguridad: se rechaza.
const (
	LongitudMinimaPassword = 8
	LongitudMaximaPassword = 72
)

// ErrPoliticaPassword indica que la contraseña no cumple la política.
var ErrPoliticaPassword = errors.New(
	"la contraseña debe tener entre 8 y 72 bytes, al menos una mayúscula, una minúscula y un dígito")

// ValidarPassword aplica la política. La contraseña se evalúa tal cual llega:
// nunca se recortan espacios, para que lo que se guarda sea exactamente lo que el
// usuario escribe al iniciar sesión.
func ValidarPassword(password string) error {
	if len(password) < LongitudMinimaPassword || len(password) > LongitudMaximaPassword {
		return ErrPoliticaPassword
	}
	var mayuscula, minuscula, digito bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			mayuscula = true
		case unicode.IsLower(r):
			minuscula = true
		case unicode.IsDigit(r):
			digito = true
		}
	}
	if !mayuscula || !minuscula || !digito {
		return ErrPoliticaPassword
	}
	return nil
}
