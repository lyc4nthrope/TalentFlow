package config

import (
	"testing"
	"time"
)

func TestSinJWTSecretNoArranca(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	if _, err := Cargar(); err == nil {
		t.Fatal("cargó la configuración sin JWT_SECRET")
	}
}

func TestVigenciasPorDefectoYConfigurables(t *testing.T) {
	t.Setenv("JWT_SECRET", "s")
	t.Setenv("TOKEN_RECUPERACION_VIGENCIA", "30m")
	cfg, err := Cargar()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JWTVigencia != time.Hour || cfg.VigenciaActivacion != 24*time.Hour || cfg.VigenciaRecuperacion != 30*time.Minute {
		t.Fatalf("vigencias = %v %v %v", cfg.JWTVigencia, cfg.VigenciaActivacion, cfg.VigenciaRecuperacion)
	}
}

func TestVigenciaInvalidaNoArranca(t *testing.T) {
	t.Setenv("JWT_SECRET", "s")
	for _, v := range []string{"mañana", "-1h", "0s"} {
		t.Setenv("TOKEN_ACTIVACION_VIGENCIA", v)
		if _, err := Cargar(); err == nil {
			t.Fatalf("aceptó la vigencia %q", v)
		}
	}
}
