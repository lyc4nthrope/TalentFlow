// Package testutil prepara una base PostgreSQL real para las pruebas de integración.
// Las pruebas se omiten si TEST_DATABASE_URL no está definida.
package testutil

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "github.com/lib/pq"
)

func Conectar(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no definida: se omiten las pruebas de integración")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatalf("abriendo BD: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, archivo, _, _ := runtime.Caller(0)
	esquema, err := os.ReadFile(filepath.Join(filepath.Dir(archivo), "..", "..", "init.sql"))
	if err != nil {
		t.Fatalf("leyendo init.sql: %v", err)
	}
	if _, err := db.Exec(string(esquema)); err != nil {
		t.Fatalf("aplicando init.sql: %v", err)
	}
	if _, err := db.Exec(`TRUNCATE perfiles, eventos_procesados`); err != nil {
		t.Fatalf("limpiando tablas: %v", err)
	}
	return db
}
