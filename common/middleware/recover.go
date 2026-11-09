package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover turns a panic in a handler into a 500 response and an error log with the stack trace.
// Without it, net/http only drops the connection and the client gets no answer.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v) // deliberate abort: let net/http handle it
				}

				slog.ErrorContext(r.Context(), "panic while serving request",
					"method", r.Method, "path", r.URL.Path, "panic", v, "stack", string(debug.Stack()))
				rw.WriteHeader(http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(rw, r)
	})
}
