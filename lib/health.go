package lib

import "net/http"

// Used by k8s to check when the app is healthy.
//
// If fails, the app will be restarted.
func Live(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// Used by k8s to check when the app is ready to serve traffic.
//
// If fails, the app won't receive any traffic until the problem is fixed.
func Ready(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// Used by k8s to check when the app has started.
//
// Similar to [ReadyHandler] but checked only when the app starts.
func Start(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
