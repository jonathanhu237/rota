package handler

import "net/http"

func New() http.Handler {
	mux := http.NewServeMux()
	return mux
}
