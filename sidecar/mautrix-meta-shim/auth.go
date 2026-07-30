package main

import "net/http"

// requireAuth wraps a handler to enforce Bearer token auth.
// Returns 401 if the header is missing or doesn't match.
func requireAuth(expectedToken string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !VerifyBearer(r.Header.Get("Authorization"), expectedToken) {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
