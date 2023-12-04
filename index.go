// Index actions
package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type OutGetterType string

func ContainsAny(str string, substr string) bool {
	return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
}

func (o *OutGetterType) Write(p []byte) (n int, err error) {
	go getVersion(p, *o)
	return
}

func getVersion(p []byte, o OutGetterType) {
	Outs := strings.Split(string(p), "\n")
	var version string
	for _, line := range Outs {
		if ContainsAny(line, "version") && !(ContainsAny(line, "go ") || ContainsAny(line, "GoVersion")) {
			var bef string
			for i, char := range line {
				letter := string(char)
				after1 := line[i+1 : i+2]
				bef += letter
				if ContainsAny(bef, "version") {
					if after1 != ":" {
						version = line[i+2:]
						break
					}
				}
			}
			break
		}
	}
	AppVersions[string(o)] = version
}

var AppVersions = map[string]string{}

func index(w http.ResponseWriter, r *http.Request) {

	w.Header().Add("X-Frame-Options", "SAMEORIGIN")

	valid, login := checkSession(w, r)
	if valid {
		var indexTemplate IndexTemplate
		indexTemplate.Login = login
		indexTemplate.Version = VERSION
		indexTemplate.Linuxuser = getLinuxUser()
		var list []AppInfo

		if r.FormValue("upload") != "" {
			upload(w, r, &indexTemplate)
		}
		if r.FormValue("start") == "Start" {
			fmt.Fprint(w, "<html>")
			startApp(r, &indexTemplate)
			indexTemplate.NeedRefresh = false
		}
		if r.FormValue("stop") == "Stop" {
			stopApp(r, &indexTemplate)
			indexTemplate.NeedRefresh = false

		}
		if r.FormValue("action") == "replace" {
			replaceApp(r, &indexTemplate)
		}
		if r.FormValue("remove") == "true" {
			appname := r.FormValue("appname")
			err := os.Remove(getAppDir() + "shelf.dir/" + appname)
			if err != nil {
				writeToLog("Error in remove: " + err.Error())
				indexTemplate.Message = "Error :" + err.Error()
				indexTemplate.Class = "errormessage"
			} else {
				indexTemplate.Message = "file removed: " + appname
				indexTemplate.Class = "infomessage"
			}
		} else if r.FormValue("remove") == "remove" {
			indexTemplate.Remove = r.FormValue("appname")
		}

		list = listApplications(w, r)
		shelfList := listShelfApplications(w, r)
		for i, item := range list {
			for _, shelfItem := range shelfList {
				if item.Filename == shelfItem.FileName {
					list[i].Class = "inShelf"
				}
			}
		}
		indexTemplate.Apps = list
		indexTemplate.ShelfApps = shelfList

		err := mytemplate.ExecuteTemplate(w, "index.html", indexTemplate)
		if err != nil {
			w.Write([]byte("Error: " + err.Error()))
		}

	}
}

func replaceApp(r *http.Request, indexTemplate *IndexTemplate) {

	appname := r.FormValue("appname")
	sourceFile := getAppDir() + "shelf.dir/" + appname
	destFile := getAppDir() + appname + "/" + appname
	isAlreadyRunning := stopIfRunning(appname, false)
	err := copyFile(sourceFile, destFile)
	if err == nil {

		indexTemplate.Message = "Application replaced: " + appname

		indexTemplate.Class = "bluemessage"
		err = os.Remove(sourceFile)
		if err != nil {
			writeToLog("Error replaceApp remove source: " + err.Error())
		}
	} else {

		indexTemplate.Message = "Error replacing file: " + appname + ": " + err.Error()
		indexTemplate.Class = "errormessage"
	}
	writeToLog(indexTemplate.Message)
	if isAlreadyRunning {
		runApp(appname)

	}
}

type StartStopType int

const (
	START StartStopType = 0
	STOP  StartStopType = 1
)

func startApp(r *http.Request, indexTemplate *IndexTemplate) {

	startAndstopApp(START, r, indexTemplate)

}

func stopApp(r *http.Request, indexTemplate *IndexTemplate) {

	startAndstopApp(STOP, r, indexTemplate)

}

func isActionHappening(actionType StartStopType, appname string) bool {

	if actionType == START {
		isrunning, _ := isAppRunning(appname)
		return isrunning
	} else {
		isrunning, _ := isAppRunning(appname) // == isClosed
		return !isrunning
	}

}

func getInfoFilename(appname string) (infoFilename string) {

	infoFilename = getAppDir() + appname + "/" + appname + ".json"
	return
}

func startAndstopApp(actionType StartStopType, r *http.Request, indexTemplate *IndexTemplate) {

	appname := r.FormValue("appname")
	var label string
	if actionType == START {
		label = "started"
		indexTemplate.Class = "infomessage"
	} else {
		label = "stopped"
		indexTemplate.Class = "redmessage"
	}
	if isActionHappening(actionType, appname) {
		indexTemplate.Message = appname + " is already " + label
		indexTemplate.Class = "warnmessage"
	} else {
		var out string
		var err string

		_, details := readAppConfig(appname)
		details.StatusTime = time.Now()
		details.Counter = 0

		if actionType == START {
			details.IsRunning = true
			details.LastStatus = "manual start"
			errorMsg := runApp(appname)
			if errorMsg != "" {
				indexTemplate.Message = "Error: " + errorMsg
				indexTemplate.Class = "errormessage"
				details.IsRunning = false
			}
		} else {
			out, err = executeKill(appname)
			details.IsRunning = false
			details.LastStatus = "manual stop"
		}
		setConfigFile(details, appname)

		if err == "" {

			indexTemplate.Message = appname + " has " + label + "\n" + out
			isrunning, _ := isAppRunning(appname)
			if label == "started" && !isrunning {
				indexTemplate.Message = "error while running " + appname + "\n" + out
				indexTemplate.Class = "errormessage"
			}
			time.Sleep(time.Second * 2)

		} else {
			indexTemplate.Message = "Error: " + err
			indexTemplate.Class = "errormessage"
		}
		writeToLog(indexTemplate.Message + ", from : " + r.RemoteAddr)
	}

}
