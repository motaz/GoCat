// GoCat project main.go
package main

import (
	"fmt"
	"html/template"
	"net/http"

	"runtime"
	"strings"
	"sync"
)

const VERSION = "1.0.48 r25-Jan"

var mytemplate *template.Template

var mutex = &sync.Mutex{}

func main() {
	println("Go version: ", runtime.Version())
	writeToLog("Starting GoCat version : " + VERSION)
	println("OS,Arch: ", runtime.GOOS, runtime.GOARCH)
	println("No of CPUs: ", runtime.NumCPU())

	checkClosedApps("GoCat start")
	go check()
	mytemplate = template.Must(template.ParseGlob("templates/*.html"))

	http.HandleFunc("/", redirectToIndex)
	http.HandleFunc("/gocat", index)
	http.HandleFunc("/gocat/", index)
	http.HandleFunc("/gocat/login", login)
	http.HandleFunc("/gocat/setup", setup)
	http.HandleFunc("/gocat/app", app)
	http.HandleFunc("/gocat/download", download)
	http.HandleFunc("/gocat/logout", logout)
	fs := SetCacheHeader(http.FileServer(http.Dir("static")))

	http.Handle("/gocat/static/", http.StripPrefix("/gocat/static/", fs))
	port := getConfigValue("port", ":2009")
	println("GoCat version: ", VERSION)
	println("Listening on port: ", port)
	if !strings.Contains(port, ":") {
		port = ":" + port
	}
	println("http://localhost" + port)

	err := http.ListenAndServe(port, nil)
	if err != nil {
		writeToLog("Error: " + err.Error())
		fmt.Println("Error: ", err.Error())
	}

}

func SetCacheHeader(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Frame-Options", "SAMEORIGIN")

		h.ServeHTTP(w, r)
	})
}

func redirectToIndex(w http.ResponseWriter, r *http.Request) {

	w.Header().Add("Content-Type", "text/html")
	w.Header().Add("encoding", "UTF-8")
	http.Redirect(w, r, "/gocat", http.StatusPermanentRedirect)
}
