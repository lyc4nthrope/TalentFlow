package api

import (
	"context"
	"fmt"
	"log/slog"
)

// registrar deja un log con el ctx de la petición o del evento, para que lleve su traceId
// en el JSON (Reto 8, F8). registrarError es lo mismo con nivel ERROR.
func registrar(ctx context.Context, formato string, args ...any) {
	slog.InfoContext(ctx, fmt.Sprintf(formato, args...))
}

func registrarError(ctx context.Context, formato string, args ...any) {
	slog.ErrorContext(ctx, fmt.Sprintf(formato, args...))
}
