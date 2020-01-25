// GoCat project main.go
package main

import (
	"html/template"
	"net/http"
	"strings"
)

var mytemplate *template.Template

func main() {

	mytemplate = template.Must(template.ParseGlob("templates/*.html"))

	http.HandleFunc("/", index)
	http.HandleFunc("/login", login)
	http.HandleFunc("/setup", setup)
	http.HandleFunc("/app", app)
	http.HandleFunc("/download", download)
	http.HandleFunc("/logout", logout)
	fs := http.FileServer(http.Dir("static"))

	http.Handle("/static/", http.StripPrefix("/static/", fs))
	port := getConfigValue("port", ":2009")
	println("Listening on port: ", port)
	if !strings.Contains(port, ":") {
		port = ":" + port
	}
	err := http.ListenAndServe(port, nil)
	if err != nil {
		println("Error: " + err.Error())
	}

}
