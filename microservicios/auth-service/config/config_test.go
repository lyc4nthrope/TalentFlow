package config

import (
	"net/url"
	"strconv"
	"strings"
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
	conBase(t)
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
	conBase(t)
	for _, v := range []string{"mañana", "-1h", "0s"} {
		t.Setenv("TOKEN_ACTIVACION_VIGENCIA", v)
		if _, err := Cargar(); err == nil {
			t.Fatalf("aceptó la vigencia %q", v)
		}
	}
}

// Reto 5, D23: sin credenciales de BD o del broker en el código.
func conBase(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "s")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("AMQP_URL", "")
	t.Setenv("DB_USER", "u")
	t.Setenv("DB_PASS", "p")
	t.Setenv("RABBITMQ_USER", "ru")
	t.Setenv("RABBITMQ_PASS", "rp")
}

func TestSinCredencialesNoArranca(t *testing.T) {
	for _, falta := range []string{"DB_USER", "DB_PASS", "RABBITMQ_USER", "RABBITMQ_PASS"} {
		for _, valor := range []string{"", "   "} {
			t.Run(falta+"="+strconv.Quote(valor), func(t *testing.T) {
				conBase(t)
				t.Setenv(falta, valor)
				if _, err := Cargar(); err == nil || !strings.Contains(err.Error(), falta) {
					t.Fatalf("%s=%q: err = %v", falta, valor, err)
				}
			})
		}
	}
}

func TestLasURLCompletasReemplazanALasCredenciales(t *testing.T) {
	conBase(t)
	for _, v := range []string{"DB_USER", "DB_PASS", "RABBITMQ_USER", "RABBITMQ_PASS"} {
		t.Setenv(v, "")
	}
	t.Setenv("DATABASE_URL", "postgres://a:b@h:5432/d")
	t.Setenv("AMQP_URL", "amqp://a:b@h:5672/")
	cfg, err := Cargar()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "postgres://a:b@h:5432/d" || cfg.AMQPURL != "amqp://a:b@h:5672/" {
		t.Fatalf("urls = %q %q", cfg.DatabaseURL, cfg.AMQPURL)
	}
}

// Una contraseña con caracteres especiales se codifica en la URL en vez de romperla.
func TestLasCredencialesSeCodificanEnLaURL(t *testing.T) {
	conBase(t)
	t.Setenv("DB_PASS", "p@ss:w/rd")
	t.Setenv("DB_HOST", "database-auth")
	cfg, err := Cargar()
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	clave, _ := u.User.Password()
	if clave != "p@ss:w/rd" || u.Host != "database-auth:5432" || u.Path != "/db_auth" || u.Query().Get("sslmode") != "disable" {
		t.Fatalf("url = %q", cfg.DatabaseURL)
	}
}

func TestJWTSecretDeSoloEspaciosNoArranca(t *testing.T) {
	conBase(t)
	for _, v := range []string{" ", "   ", "\t\n"} {
		t.Setenv("JWT_SECRET", v)
		if _, err := Cargar(); err == nil {
			t.Fatalf("cargó con JWT_SECRET=%q", v)
		}
	}
}

// Los hosts se unen con su puerto con net.JoinHostPort: una IPv6 queda entre corchetes.
func TestHostIPv6EnLasURLs(t *testing.T) {
	conBase(t)
	t.Setenv("DB_HOST", "::1")
	t.Setenv("RABBITMQ_HOST", "fd00::5")
	cfg, err := Cargar()
	if err != nil {
		t.Fatal(err)
	}
	bd, err := url.Parse(cfg.DatabaseURL)
	if err != nil || bd.Host != "[::1]:5432" || bd.Hostname() != "::1" || bd.Port() != "5432" {
		t.Fatalf("url BD = %q (err %v)", cfg.DatabaseURL, err)
	}
	mq, err := url.Parse(cfg.AMQPURL)
	if err != nil || mq.Host != "[fd00::5]:5672" {
		t.Fatalf("url AMQP = %q (err %v)", cfg.AMQPURL, err)
	}
}
