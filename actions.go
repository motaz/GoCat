package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/motaz/codeutils"
)

type HeaderValues struct {
	Version   string
	Message   string
	Class     string
	Login     string
	Linuxuser string
	Hostname  string
}

type IndexTemplate struct {
	HeaderValues
	NeedRefresh bool

	Remove    string
	Apps      []AppInfo
	ShelfApps []ShelfAppInfo
}

type DetailFile struct {
	AppName    string
	IsRunning  bool
	LastStatus string
	StatusTime time.Time
	Port       string
	Counter    int
}

type SessionType struct {
	Username string
	Hash     string
	Expiary  time.Time
}

func getHash(userAgent, username, ip string) (hash string) {

	if strings.Contains(ip, ".") {
		ip = ip[:strings.LastIndex(ip, ".")]
	} else if strings.Contains(ip, ":") && len(ip) > 30 {
		ip = ip[:30]
	}
	hash = codeutils.GetMD5(userAgent + username + ip)
	return
}

func saveSession(r *http.Request, sessionValue string, keep bool) (err error) {

	dir := codeutils.GetCurrentAppDir() + "/sessions"
	if !codeutils.IsFileExists(dir) {
		os.Mkdir(dir, os.ModePerm)
	}
	username := r.FormValue("login")
	filename := dir + "/" + sessionValue
	var session SessionType
	session.Username = username
	ip := codeutils.GetRemoteIP(r)
	agent := r.UserAgent()
	session.Hash = getHash(agent, username, ip)
	if keep {
		session.Expiary = time.Now().AddDate(0, 1, 0)
	} else {
		session.Expiary = time.Now().Add(time.Hour * 8)
	}
	data, _ := json.Marshal(session)
	err = writeToFile(filename, data)

	return
}

func readSession(sessionValue string) (session SessionType, err error) {

	var data []byte
	filename := codeutils.GetCurrentAppDir() + "/sessions/" + sessionValue
	data, err = os.ReadFile(filename)
	if err == nil {
		err = json.Unmarshal(data, &session)
		if err == nil {
			if time.Now().After(session.Expiary) {
				err = errors.New("Session has expired")
			}

		}
	}
	return

}

func removeSession(sessionValue string) {

	filename := codeutils.GetCurrentAppDir() + "/sessions/" + sessionValue
	os.Remove(filename)
}

// backward compatability, temporary
func readOldSession(w http.ResponseWriter, r *http.Request) (valid bool, login string) {

	writeToLog("Old session")
	sessionCookie, err := r.Cookie("gocatsession")
	loginCookie, err2 := r.Cookie("login")
	valid = false
	if err == nil || err2 == nil {
		var asession string
		var currentSession string
		if sessionCookie != nil && loginCookie != nil {
			currentSession = sessionCookie.Value
			currentUser := loginCookie.Value
			asession = GetMD5Hash(currentUser + "9012")
			valid = asession == currentSession

		}
		if err2 == nil {
			login = loginCookie.Value
		}
	}
	return
}

func checkSession(w http.ResponseWriter, r *http.Request) (valid bool, username string) {

	sessionCookie, err := r.Cookie("gocatsession")
	valid = err == nil
	if valid {
		var session SessionType
		session, err = readSession(sessionCookie.Value)
		valid = err == nil
		if !valid {
			writeToLog("Error in reading file checkSession: " + err.Error())
		} else if session.Hash != "" { // this condition should be removed
			ip := codeutils.GetRemoteIP(r)
			hash := getHash(r.UserAgent(), session.Username, ip)
			valid = hash == session.Hash

		}
		if valid {
			username = session.Username

		}

	} else {
		writeToLog("Error in reading cookies checkSession: " + err.Error())
	}
	if !valid {
		valid, username = readOldSession(w, r) // should be removed in newer versions
	}

	if !valid {
		writeToLog("Invalid session, redrecting to login, from: " + r.RemoteAddr)

		http.Redirect(w, r, "/gocat/login", http.StatusTemporaryRedirect)
	}

	return
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
			keep := r.FormValue("keeplogin") == "1"

			if checkLogin(r.FormValue("login"), r.FormValue("password")) {
				setLoginCookies(w, r, keep)
				w.Write([]byte("<script>document.location='/gocat/';</script>"))
				writeToLog("Successful login for " + r.FormValue("login") + ", from: " +
					r.RemoteAddr)

			} else {
				result.ErrorMsg = "Invalid username/and or password"
				result.IsInvalid = true
				writeToLog("Invalid login for: " + r.FormValue("login") + ", from: " +
					r.RemoteAddr)
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

func setLoginCookies(w http.ResponseWriter, r *http.Request, keepSession bool) {

	var expiration time.Time

	if r.FormValue("keeplogin") == "1" {
		expiration = time.Now().AddDate(0, 1, 0)

	} else {
		expiration = time.Now().Add(time.Hour * 8)

	}
	sessionValue := GetMD5Hash(r.UserAgent() + time.Now().String())
	saveSession(r, sessionValue, keepSession)
	cookie := http.Cookie{Name: "gocatsession", Value: sessionValue, Expires: expiration}
	http.SetCookie(w, &cookie)
}

func checkLogin(username string, userpassword string) bool {

	user := codeutils.GetConfigValue("gocat.ini", "user")

	configpassword := codeutils.GetConfigValue("gocat.ini", "password")
	hashpassword := GetMD5Hash(userpassword)

	return (user == username) && (configpassword == hashpassword)

}

type ApplicationInfo struct {
	HeaderValues
	AppName     string
	Port        string
	Dir         string
	IsSubFolder bool
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

	valid, login := checkSession(w, r)
	if valid {
		var applicationInfo ApplicationInfo
		applicationInfo.Login = login
		applicationInfo.Hostname, _ = os.Hostname()
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

		err := mytemplate.ExecuteTemplate(w, "application.html", applicationInfo)
		if err != nil {
			w.Write([]byte("Error: " + err.Error()))
		}
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
	valid, _ := checkSession(w, r)
	if valid {
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
}

func logout(w http.ResponseWriter, r *http.Request) {

	w.Header().Add("Content-Type", "text/html;charset=UTF-8")
	w.Header().Add("encoding", "UTF-8")
	sessionCookie, err := r.Cookie("gocatsession")
	if err == nil {
		removeSession(sessionCookie.Value)
		expiration := time.Now()
		cookie := http.Cookie{Name: "gocatsession", Value: "-", Expires: expiration}
		http.SetCookie(w, &cookie)
	}

	http.Redirect(w, r, "/gocat/login", http.StatusTemporaryRedirect)

}
