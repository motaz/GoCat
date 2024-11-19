// GoCat project main.go
package main

import (
	"fmt"
	"html/template"
	"net/http"

	"embed"
	"runtime"
	"strings"
)

const VERSION = "1.0.56 r19-Nov"

var mytemplate *template.Template

//go:embed templates
var templates embed.FS

//go:embed static
var static embed.FS

func InitTemplate(embededTemplates embed.FS) (err error) {

	mytemplate, err = template.ParseFS(embededTemplates, "templates/*.html")
	if err != nil {
		fmt.Println("error in InitTemplate: " + err.Error())
	}
	return
}

func main() {
	fmt.Println("Go version: ", runtime.Version())
	writeToLog("Starting GoCat version : " + VERSION)
	fmt.Println("OS,Arch: ", runtime.GOOS, runtime.GOARCH)
	fmt.Println("No of CPUs: ", runtime.NumCPU())

	checkClosedApps("GoCat start")
	go check()
	err := InitTemplate(templates)
	if err == nil {
		http.Handle("/gocat/static/", http.StripPrefix("/gocat/", http.FileServer(http.FS(static))))

		http.HandleFunc("/", redirectToIndex)
		http.HandleFunc("/gocat", index)
		http.HandleFunc("/gocat/", index)
		http.HandleFunc("/gocat/login", login)
		http.HandleFunc("/gocat/setup", setup)
		http.HandleFunc("/gocat/app", app)
		http.HandleFunc("/gocat/download", download)
		http.HandleFunc("/gocat/logout", logout)
		http.HandleFunc("/gocat/changepass", changePass)

		port := getConfigValue("port", ":2009")
		fmt.Println("GoCat version: ", VERSION)
		fmt.Println("Listening on port: ", port)
		if !strings.Contains(port, ":") {
			port = ":" + port
		}
		fmt.Println("http://localhost" + port)

		err = http.ListenAndServe(port, nil)
		if err != nil {
			writeToLog("Error: " + err.Error())
			fmt.Println("Error: ", err.Error())
		}
	} else {
		fmt.Println("Error in template: ", err.Error())
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
