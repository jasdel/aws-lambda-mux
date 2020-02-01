package lambdamux

import "net/http"

func StartLocalServer(addr string) error {
	mux := http.NewServeMux()

	return http.ListenAndServe(addr, mux)
}

type LocalServer struct {
}
