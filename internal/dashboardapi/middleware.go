// middleware.go: CORS la cross-cutting concern -> boc mot lan o bien ngoai cung
// cua router thay vi lap lai tren tung route. Nho vay ca response do chinh
// ServeMux sinh ra (301 canonical-path, 404, 405) cung mang header CORS, browser
// doc duoc status that thay vi bao loi CORS mo ho.
package dashboardapi

import "net/http"

func WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Control-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
