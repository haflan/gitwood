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

func logPageTmplErr(page string, err error) {
	if err == nil {
		return
	}
	log.Println("failed to execute", page, ":", err)
}

func (pc *PageContext) errorRequest(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusBadRequest)
	logPageTmplErr("err_request", errorTmpl.Execute(w, ErrorPageData{
		PageContext: *pc,
		Code:        http.StatusBadRequest,
		Message:     message,
	}))
}

func (pc *PageContext) errorPageNotFound(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusNotFound)
	logPageTmplErr("err_not_found", errorTmpl.Execute(w, ErrorPageData{
		PageContext: *pc,
		Code:        http.StatusNotFound,
		Message:     message,
	}))
}

func (pc *PageContext) errorPageServer(w http.ResponseWriter, message string, err error) {
	log.Println("internal server error:", err)
	w.WriteHeader(http.StatusInternalServerError)
	logPageTmplErr("err_server", errorTmpl.Execute(w, ErrorPageData{
		PageContext: *pc,
		Code:        http.StatusInternalServerError,
		Message:     message,
	}))
}
