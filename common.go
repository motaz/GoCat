// common
package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/motaz/codeutils"
)

type FileInfo struct {
	FileName string
	Size     string
	FileTime string
	IsDir    bool
	IsEdit   bool
}

func writeToFile(filename string, contents string) error {
	err := ioutil.WriteFile(filename, []byte(contents), 0644)
	return err

}

func getAppDir() string {
	dir := codeutils.GetCurrentAppDir() + "/apps/"
	return dir
}

func initApplications() {
	list, err := readRunningApps()
	if err == nil {
		for _, appName := range list {
			if strings.Trim(appName, "") != "" {
				isRunning, _ := isAppRunning(appName)
				if isRunning {
					writeToLog(appName + " is already running")
				} else {
					runApp(appName)
					writeToLog("Starting: " + appName)
				}
			}
		}
	}
}

func getConfigValue(valuename string, defaultvalue string) (value string) {

	value = codeutils.GetConfigValue("gocat.ini", valuename)
	if value == "" {
		value = defaultvalue
	}
	return
}

type AppInfo struct {
	Filename     string
	Port         string
	Running      string
	Address      string
	FileTime     string
	IsRunning    bool
	RunningSince string
	SinceColor   string
}

func getAppList() (apps []DetailFile) {

	dir := getAppDir()
	files, err := ioutil.ReadDir(dir)

	apps = make([]DetailFile, 0)
	if err == nil {
		for _, f := range files {
			if f.IsDir() {
				fullfilename := dir + f.Name() + "/" + f.Name()
				if err == nil {

					hasJson, info := readAppConfig(fullfilename + ".json")
					info.AppName = f.Name()
					if hasJson {
						apps = append(apps, info)
					}
				}
			}
		}
	}
	return
}

func listApplications(w http.ResponseWriter, r *http.Request) []AppInfo {

	dir := getAppDir()
	files, err := ioutil.ReadDir(dir)

	if err == nil {

		address := "http://" + r.Host
		if strings.Contains(r.Host, ":") {
			address = address[:strings.LastIndex(address, ":")]
		}

		file, fileerror := os.Create("running.txt")
		var writer *bufio.Writer
		if fileerror == nil {

			defer file.Close()

			writer = bufio.NewWriter(file)
			defer writer.Flush()
		}
		var list []AppInfo
		for _, f := range files {
			if f.IsDir() {
				var afile AppInfo
				afile.Filename = f.Name()
				fullfilename := dir + afile.Filename + "/" + afile.Filename
				hasJson := false
				fileInfo, err := os.Stat(fullfilename)
				//port := ""
				if err == nil {

					afile.FileTime = fileInfo.ModTime().String()
					afile.FileTime = afile.FileTime[:19]
					var info DetailFile
					hasJson, info = readAppConfig(fullfilename + ".json")
					afile.Port = info.Port
				}

				if hasJson {

					afile.Address = address + ":" + afile.Port
					afile.IsRunning, afile.RunningSince = isAppRunning(afile.Filename)
					if strings.Contains(afile.RunningSince, ":") {
						afile.SinceColor = "blue"
					} else {
						afile.SinceColor = "black"
					}
					if afile.IsRunning {
						afile.Running = "Running"

						if fileerror == nil {
							writer.WriteString(afile.Filename + "\n")
						}
					} else {
						afile.Running = "Stopped"
					}
					list = append(list, afile)
				}

			}
		}

		return list
	} else {
		return nil
	}

}

type ShelfAppInfo struct {
	FileName string
	FileTime string
	FileSize string
}

func listShelfApplications(w http.ResponseWriter, r *http.Request) []ShelfAppInfo {

	dir := getAppDir() + "shelf.dir"
	files, err := ioutil.ReadDir(dir)
	var list []ShelfAppInfo
	if err == nil {

		for _, f := range files {
			if !f.IsDir() {
				var shelfFile ShelfAppInfo
				shelfFile.FileName = f.Name()
				shelfFile.FileTime = f.ModTime().String()[:19]
				shelfFile.FileSize = displaySize(f.Size())
				list = append(list, shelfFile)
			}

		}
	}

	return list

}

func displaySize(size int64) (sizeText string) {

	if size < 1024 {
		sizeText = fmt.Sprintf("%d B", size)
	} else if size < 1024*1024 {
		sizeText = fmt.Sprintf("%0.1f K", float64(size)/1000)
	} else {
		sizeText = fmt.Sprintf("%0.1f M", (float32(size)/1000)/1000)

	}
	return
}

func listFiles(dir string, w http.ResponseWriter) []FileInfo {

	files, _ := ioutil.ReadDir(dir)

	var list []FileInfo
	for _, f := range files {

		var afile FileInfo
		afile.FileName = f.Name()

		afile.FileTime = f.ModTime().String()
		if !f.IsDir() {
			afile.Size = displaySize(f.Size())
		}

		afile.IsDir = f.IsDir()

		ext := filepath.Ext(f.Name())
		afile.IsEdit = !afile.IsDir && ext != ""
		list = append(list, afile)

	}

	return list
}

func readAppConfig(jsonfilename string) (success bool, info DetailFile) {

	success = false

	contents, err := ioutil.ReadFile(jsonfilename)

	if err != nil {

		return
	} else {
		err := json.Unmarshal(contents, &info)
		if err != nil {
			println("Error in getPort: ", err.Error())
		}
		success = true
		return
	}

}

func isAppRunning(appname string) (isRunning bool, since string) {

	var out bytes.Buffer

	cmd := exec.Command("/bin/bash", "-c", "ps -ef | grep "+appname)
	cmd.Stdout = &out
	cmd.Run()

	isRunning = false

	lines := strings.Split(out.String(), "\n")
	for i := 0; i < len(lines); i++ {
		if (strings.Contains(lines[i], appname)) && (!strings.Contains(lines[i], "grep")) &&
			(!strings.Contains(lines[i], "check")) {
			since = lines[i]
			for j := 0; j < 4; j++ {
				if strings.Contains(since, " ") {

					since = strings.Trim(since[strings.Index(since, " "):], " ")
				}

			}
			if strings.Contains(since, " ") {
				since = since[:strings.Index(since, " ")]
			}

			isRunning = true
		}
	}

	return
}

const (
	Run   int = 0
	Start int = 1
)

func executeKill(appname string) (result string, errorMsg string) {

	result, errorMsg = runShell(Run, "/bin/sh", "-c", "killall "+appname)
	return
}

func Shell(command string) (result string, errorMsg string) {

	result, errorMsg = runShell(Start, "/bin/sh", command)

	return
}

func runShell(runOrStart int, command string, arguments ...string) (result string, errorMsg string) {

	var out bytes.Buffer
	var err bytes.Buffer

	cmd := exec.Command(command, arguments...)
	cmd.Stdout = &out
	cmd.Stderr = &err
	if runOrStart == Run {
		cmd.Run()
	} else if runOrStart == Start {
		cmd.Start()
	}
	result = out.String()
	errorMsg = err.String()
	return
}

func getLinuxUser() string {
	result, err := runShell(Run, "/bin/sh", "-c", "whoami")

	if err != "" {
		println("Error: " + err)
	}
	return result
}

func GetMD5Hash(text string) string {

	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

func copyFile(sourcename string, targetname string) error {

	source, err := os.Open(sourcename)

	if err == nil {
		defer source.Close()

		os.Remove(targetname)
		target, err := os.OpenFile(targetname, os.O_WRONLY|os.O_CREATE, 0766)
		if err == nil {
			defer target.Close()
			_, err = io.Copy(target, source)
		}

	}
	return err
}

func runApp(appname string) {

	filename := getAppDir() + appname + "/start.sh"
	Shell(filename)
}

func stopIfRunning(filename string, toShelf bool) (isAlreadyRunning bool) {

	isAlreadyRunning, _ = isAppRunning(filename)
	if !toShelf && isAlreadyRunning {
		executeKill(filename)
		time.Sleep(time.Second * 4)
	}
	return
}

func writeToLog(event string) {
	codeutils.WriteToLog(event, "gocat")
}

func readRunningApps() (list []string, err error) {

	var content []byte
	content, err = ioutil.ReadFile("running.txt")
	if err == nil {

		list = strings.Split(string(content), "\n")
	}
	return
}

func checkClosedApps() {

	list := getAppList()
	for _, item := range list {
		if item.IsRunning {

			isRunning, _ := isAppRunning(item.AppName)
			if !isRunning {
				runApp(item.AppName)
				writeToLog("Starting after close/crash: " + item.AppName)
			}

		}
	}
}

func check() {
	for {
		time.Sleep(time.Minute * 2)
		checkClosedApps()
	}
}
