// Index actions
package main

import (
	"net/http"
	"os"
	"time"
)

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

		list = listApplications(w, r)
		shelfList := listShelfApplications(w, r)
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
		os.Remove(sourceFile)
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
		infoFilename := getAppDir() + r.FormValue("appname") + "/" + r.FormValue("appname") + ".json"
		_, details := readAppConfig(infoFilename)

		if actionType == START {
			details.IsRunning = true
			out, err = Shell(getAppDir() + r.FormValue("appname") + "/start.sh")
		} else {
			out, err = executeKill(r.FormValue("appname"))
			details.IsRunning = false
		}
		setConfigFile(details, infoFilename)
		println(infoFilename)

		if err == "" {

			indexTemplate.Message = appname + " has " + label + "\n" + out
			time.Sleep(time.Second * 2)

		} else {
			indexTemplate.Message = "Error: " + err
			indexTemplate.Class = "errormessage"
		}
		writeToLog(indexTemplate.Message + ", from : " + r.RemoteAddr)
	}

}
