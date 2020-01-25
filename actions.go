package main

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/motaz/codeutils"
)

type IndexTemplate struct {
	Message     string
	Class       string
	NeedRefresh bool
	Login       string
	Linuxuser   string
	Apps        []AppInfo
	ShelfApps   []string
}

type DetailFile struct {
	Port string
}

func checkSession(w http.ResponseWriter, r *http.Request) (bool, string) {

	sessionCookie, err := r.Cookie("gocatsession")
	loginCookie, err2 := r.Cookie("login")
	login := "="
	valid := false
	if err == nil || err2 == nil {
		var asession string
		var currentSession string
		if sessionCookie != nil && loginCookie != nil {
			currentSession = sessionCookie.Value
			currentUser := loginCookie.Value
			asession = GetMD5Hash(currentUser + "9012")
			valid = asession == currentSession

		}
		login = loginCookie.Value

	}
	if !valid {

		http.Redirect(w, r, "/login", 307)
	}

	return valid, login
}

type OutputData struct {
	IsInvalid bool
	ErrorMsg  string
	Login     string
}

func login(w http.ResponseWriter, r *http.Request) {

	user := getConfigValue("user", "")
	if user == "" {
		http.Redirect(w, r, "/setup", 307)

	} else {
		var result OutputData
		result.IsInvalid = false
		if r.FormValue("submitlogin") != "" {
			if checkLogin(r.FormValue("login"), r.FormValue("password")) {
				setCookies(w, r)
				w.Write([]byte("<script>document.location='/';</script>"))

			} else {
				result.ErrorMsg = "Invalid username/and or password"
				result.IsInvalid = true
			}
		}
		err := mytemplate.ExecuteTemplate(w, "login.html", result)
		if err != nil {
			w.Write([]byte("Error: " + err.Error()))
		}
	}
}

func setup(w http.ResponseWriter, r *http.Request) {

	if codeutils.IsFileExists("gocat.ini") {
		http.Redirect(w, r, "/login", 307)

	} else {
		var result OutputData
		result.IsInvalid = false
		if r.FormValue("setup") != "" {
			if (r.FormValue("password") == "") ||
				(r.FormValue("confirmpassword") != r.FormValue("password")) {

			} else {
				codeutils.SetConfigValue("gocat.ini", "user", r.FormValue("login"))
				passwordhash := GetMD5Hash(r.FormValue("password"))
				codeutils.SetConfigValue("gocat.ini", "password", passwordhash)
				http.Redirect(w, r, "/login", 307)

			}
		}
		err := mytemplate.ExecuteTemplate(w, "setup.html", result)
		if err != nil {
			w.Write([]byte("Error: " + err.Error()))
		}
	}
}

func setCookies(w http.ResponseWriter, r *http.Request) {

	expiration := time.Now().Add(time.Hour * 24)
	sessionValue := GetMD5Hash(r.FormValue("login") + "9012")
	cookie := http.Cookie{Name: "gocatsession", Value: sessionValue, Expires: expiration}

	loginCookie := http.Cookie{Name: "login", Value: r.FormValue("login"), Expires: expiration}
	http.SetCookie(w, &cookie)
	http.SetCookie(w, &loginCookie)
}

func checkLogin(username string, userpassword string) bool {

	user := codeutils.GetConfigValue("gocat.ini", "user")

	configpassword := codeutils.GetConfigValue("gocat.ini", "password")
	hashpassword := GetMD5Hash(userpassword)

	return (user == username) && (configpassword == hashpassword)

}

type ApplicationInfo struct {
	AppName     string
	Dir         string
	Message     string
	Login       string
	IsSubFolder bool
	Linuxuser   string
	Files       []FileInfo

	HasUpload     bool
	UploadedFiles []UploadedFileInfo
}

func app(w http.ResponseWriter, r *http.Request) {

	_, login := checkSession(w, r)
	var applicationInfo ApplicationInfo
	applicationInfo.Login = login
	applicationInfo.HasUpload = false
	applicationInfo.Linuxuser = getLinuxUser()

	if r.FormValue("upload") != "" {

		applicationInfo.UploadedFiles = uploadfiles(w, r)
		if len(applicationInfo.UploadedFiles) > 0 {
			applicationInfo.HasUpload = true

		}
	}
	appname := r.FormValue("appname")
	dir := getAppDir() + appname
	files := listFiles(dir, w)

	applicationInfo.AppName = appname
	applicationInfo.Dir = dir
	applicationInfo.IsSubFolder = strings.Contains(appname, "/")

	applicationInfo.Files = files
	err := mytemplate.ExecuteTemplate(w, "application.html", applicationInfo)
	if err != nil {
		w.Write([]byte("Error: " + err.Error()))
	}

}

func download(w http.ResponseWriter, r *http.Request) {

	filename := r.FormValue("filename")
	filename = getAppDir() + filename
	println(filename)
	file, e := os.Open(filename)
	if e != nil {
		w.Header().Add("Content-Type", "text/html;charset=UTF-8")
		w.Header().Add("encoding", "UTF-8")

		println(e.Error())
		w.Write([]byte("Error: " + e.Error()))

	} else {
		onlyname := filename[strings.LastIndex(filename, "/"):]
		w.Header().Set("Content-Disposition", "attachment; filename="+onlyname)
		//w.Header().Set("Content-Type", "image/png")

		read := bufio.NewReader(file)

		data := make([]byte, 4096)
		for {
			numread, err := read.Read(data)
			if (err != nil) && (err == io.EOF) {
				break
			}
			w.Write(data[:numread])
		}
	}

}

func logout(w http.ResponseWriter, r *http.Request) {

	w.Header().Add("Content-Type", "text/html;charset=UTF-8")
	w.Header().Add("encoding", "UTF-8")

	expiration := time.Now()
	cookie := http.Cookie{Name: "gocatsession", Value: "-", Expires: expiration}
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/login", 307)

}
