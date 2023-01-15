package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/motaz/codeutils"
)

type IndexTemplate struct {
	Version     string
	Message     string
	Class       string
	NeedRefresh bool
	Login       string
	Linuxuser   string
	Apps        []AppInfo
	ShelfApps   []ShelfAppInfo
}

type DetailFile struct {
	AppName   string
	IsRunning bool
	Port      string
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
		writeToLog("Invalid session, redrecting to login, from: " + r.RemoteAddr)

		http.Redirect(w, r, "/gocat/login", 307)
	}

	return valid, login
}

type OutputData struct {
	IsInvalid bool
	ErrorMsg  string
	Login     string
	Version   string
}

func login(w http.ResponseWriter, r *http.Request) {

	user := getConfigValue("user", "")
	if user == "" {
		http.Redirect(w, r, "/gocat/setup", http.StatusTemporaryRedirect)

	} else {
		w.Header().Add("X-Frame-Options", "SAMEORIGIN")
		var result OutputData
		result.IsInvalid = false
		result.Version = VERSION
		if r.FormValue("submitlogin") != "" {
			if checkLogin(r.FormValue("login"), r.FormValue("password")) {
				setLoginCookies(w, r)
				w.Write([]byte("<script>document.location='/gocat/';</script>"))
				writeToLog("Successful login for " + r.FormValue("login") + ", from: " + r.RemoteAddr)

			} else {
				result.ErrorMsg = "Invalid username/and or password"
				result.IsInvalid = true
				writeToLog("Invalid login for: " + r.FormValue("login") + ", from: " + r.RemoteAddr)
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
		http.Redirect(w, r, "/gocat/login", 307)

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
				http.Redirect(w, r, "/gocat/login", 307)

			}
		}
		err := mytemplate.ExecuteTemplate(w, "setup.html", result)
		if err != nil {
			w.Write([]byte("Error: " + err.Error()))
		}
	}
}

func setLoginCookies(w http.ResponseWriter, r *http.Request) {

	var expiration time.Time

	if r.FormValue("keeplogin") == "1" {
		expiration = time.Now().AddDate(0, 1, 0)

	} else {
		expiration = time.Now().Add(time.Hour * 8)

	}
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
	Version     string
	AppName     string
	Port        string
	Dir         string
	Message     string
	Login       string
	IsSubFolder bool
	Linuxuser   string
	Files       []FileInfo

	HasUpload     bool
	UploadedFiles []UploadedFileInfo

	Editing      bool
	EditFileName string
	Content      string

	NewFile bool

	RemoveFile     bool
	RemoveFileName string

	RenameFile     bool
	RenameFileName string
}

func removeFile(dir string, w http.ResponseWriter, r *http.Request) {

	if r.FormValue("confirmremove") != "" {
		writeToLog("Removing file: " + r.FormValue("removefilename"))
		err := os.Remove(dir + "/" + r.FormValue("removefilename"))
		if err != nil {
			fmt.Fprintf(w, "<p id=errormessage>%s</p>", err.Error())
		} else {
			fmt.Fprintf(w, "<p id=infomessage>File removed: %s</p>", r.FormValue("removefilename"))
		}
	}
}

func renameFile(dir string, w http.ResponseWriter, r *http.Request) {

	if r.FormValue("dorename") != "" {
		writeToLog("Renaming file: " + r.FormValue("renamefilename"))
		err := os.Rename(dir+"/"+r.FormValue("renamefilename"), dir+"/"+r.FormValue("newfilename"))
		if err != nil {
			fmt.Fprintf(w, "<p id=errormessage>%s</p>", err.Error())
		} else {
			fmt.Fprintf(w, "<p id=infomessage>File renamed: %s</p>", r.FormValue("newfilename"))
		}
	}
}

func saveNewFile(dir string, applicationInfo *ApplicationInfo, w http.ResponseWriter, r *http.Request) {

	if r.FormValue("savenewfile") != "" {
		actualSaveFile(r.FormValue("newfilename"), dir, r.FormValue("content"), w)
	}
}

func saveFile(dir string, applicationInfo *ApplicationInfo, w http.ResponseWriter, r *http.Request) {

	if r.FormValue("save") != "" {
		actualSaveFile(r.FormValue("editfile"), dir, r.FormValue("content"), w)
		applicationInfo.Editing = false

	}
}

func actualSaveFile(filename, dir, contents string, w http.ResponseWriter) (err error) {

	writeToLog("Saving file: " + filename)
	err = WriteToFile(dir+"/"+filename, contents)
	if err != nil {
		fmt.Fprintf(w, "<p id=errormessage>%s</p>", err.Error())
	} else {
		fmt.Fprintf(w, "<p id=infomessage>File saved: %s</p>", filename)
	}
	return

}

func editFile(dir string, applicationInfo *ApplicationInfo, w http.ResponseWriter, r *http.Request) {

	if r.FormValue("edit") != "" {
		applicationInfo.Editing = true
		applicationInfo.EditFileName = r.FormValue("editfile")
		content, err := ioutil.ReadFile(dir + "/" + applicationInfo.EditFileName)
		if err != nil {
			writeToLog("Error in editFile: " + err.Error())
		}
		applicationInfo.Content = string(content)

	}

}

func showNewFile(dir string, applicationInfo *ApplicationInfo, w http.ResponseWriter, r *http.Request) {

	if r.FormValue("newfile") != "" {
		applicationInfo.NewFile = true
	}

}

func changePort(configFileName string, r *http.Request) {

	if r.FormValue("changeport") != "" {
		_, detail := readAppConfig(configFileName)

		detail.Port = r.FormValue("newport")
		setConfigFile(detail, configFileName)

	}
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
	removeFile(dir, w, r)
	renameFile(dir, w, r)
	saveNewFile(dir, &applicationInfo, w, r)

	configFileName := dir + "/" + appname + ".json"
	changePort(configFileName, r)
	_, info := readAppConfig(configFileName)

	applicationInfo.Port = info.Port

	files := listFiles(dir, w)
	applicationInfo.Version = VERSION
	applicationInfo.AppName = appname
	applicationInfo.Dir = dir
	applicationInfo.IsSubFolder = strings.Contains(appname, "/")

	applicationInfo.Files = files

	editFile(dir, &applicationInfo, w, r)
	saveFile(dir, &applicationInfo, w, r)
	showNewFile(dir, &applicationInfo, w, r)
	if r.FormValue("remove") != "" {
		applicationInfo.RemoveFile = true
		applicationInfo.RemoveFileName = r.FormValue("editfile")
	}

	if r.FormValue("rename") != "" {
		applicationInfo.RenameFile = true
		applicationInfo.RenameFileName = r.FormValue("renamefile")
	}

	changePort(configFileName, r)
	err := mytemplate.ExecuteTemplate(w, "application.html", applicationInfo)
	if err != nil {
		w.Write([]byte("Error: " + err.Error()))
	}

}

func WriteToFile(filename string, data string) error {

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.WriteString(file, data)
	if err != nil {
		return err
	}
	return file.Sync()
}

func download(w http.ResponseWriter, r *http.Request) {

	filename := r.FormValue("filename")
	filename = getAppDir() + filename

	file, e := os.Open(filename)
	if e != nil {
		w.Header().Add("Content-Type", "text/html;charset=UTF-8")
		w.Header().Add("encoding", "UTF-8")

		w.Write([]byte("Error: " + e.Error()))

	} else {
		onlyname := filename[strings.LastIndex(filename, "/"):]
		w.Header().Set("Content-Disposition", "attachment; filename="+onlyname)

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
	cookie2 := http.Cookie{Name: "login", Value: "-", Expires: expiration}
	http.SetCookie(w, &cookie2)
	http.Redirect(w, r, "/gocat/login", 307)

}
