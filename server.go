package main

import (
	"io"
	"net/http"
)

// func New()

func Server(port string, endpoints map[string]string) error {
	mux := http.NewServeMux()

	for endpoint, contents := range endpoints {
		mux.HandleFunc(endpoint, CompileEndpoint(contents))
	}

	return http.ListenAndServe(port, mux)
}

func CompileEndpoint(contents string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, contents)
	}
}
