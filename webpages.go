package main

import (
	"log"
	"net/http"
)

type ErrorPageData struct {
	PageContext
	Code    int
	Message string
}

func (pd PageContext) errorPageNotFound(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusNotFound)
	errorTmpl.Execute(w, ErrorPageData{
		PageContext: pd,
		Code:        http.StatusNotFound,
		Message:     message,
	})
}

func (pd PageContext) errorPageServer(w http.ResponseWriter, message string, err error) {
	log.Println("internal server error:", err)
	w.WriteHeader(http.StatusInternalServerError)
	errorTmpl.Execute(w, ErrorPageData{
		PageContext: pd,
		Code:        http.StatusInternalServerError,
		Message:     message,
	})
}
