package main

import (
	"fmt"
	"net/http"
	"text/template"
)

type Result struct {
	Msg  string
	Red  string
	Dest string
}

// Handling errors - all errors and messages are parsed into this function and displayed on the info page
func ErrorHandler(w http.ResponseWriter, status int, m, re, d string) {
	if status == 500 {
		http.Error(w, "500 - Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(status)

	var res Result
	res.Msg = m
	res.Red = re
	res.Dest = d

	// If page not found, display respective error page
	p, err := template.ParseFiles("forum/info.html")
	if err != nil {
		fmt.Println(err)
		ErrorHandler(w, http.StatusInternalServerError, "", "", "")
		return
	}

	p.ExecuteTemplate(w, p.Name(), res)
}
