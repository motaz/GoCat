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
	"sync"
	"time"

	"github.com/motaz/codeutils"
)

var CloseMutex = &sync.Mutex{}

type FileInfo struct {
	FileName string
	Size     string
	FileTime string
	IsDir    bool
	IsEdit   bool
}

func writeToFile(filename string, contents []byte) error {

	err := ioutil.WriteFile(filename, contents, 0644)
	return err

}

func getAppDir() string {

	dir := codeutils.GetCurrentAppDir() + "/apps/"
	return dir
}

func getConfigValue(valuename string, defaultvalue string) (value string) {

	value = codeutils.GetConfigValue("gocat.ini", valuename)
	if value == "" {
		value = defaultvalue
	}
	return
}

type AppInfo struct {
	Version      string
	Filename     string
	Class        string
	Port         string
	Running      string
	Address      string
	FileTime     string
	IsRunning    bool
	RunningSince string
	SinceColor   string
	LastStatus   string
	StatusTime   time.Time
	StatusColor  string
	TimeColor    string
}

func getAppList() (apps []DetailFile) {

	dir := getAppDir()
	files, err := ioutil.ReadDir(dir)

	apps = make([]DetailFile, 0)
	if err == nil {
		for _, f := range files {
			if f.IsDir() {
				fullfilename := getInfoFilename(f.Name())
				if err == nil {

					hasJson, info := readAppConfig(fullfilename)
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

		var list []AppInfo
		for _, f := range files {
			if f.IsDir() {
				var afile AppInfo
				afile.Filename = f.Name()
				afile.Version, _ = AppVersions[afile.Filename]
				fullfilename := dir + afile.Filename + "/" + afile.Filename
				hasJson := false
				fileInfo, err := os.Stat(fullfilename)
				if err == nil {

					afile.FileTime = fileInfo.ModTime().String()
					afile.FileTime = afile.FileTime[:19]
					var info DetailFile
					hasJson, info = readAppConfig(fullfilename + ".json")
					afile.Port = info.Port
					afile.LastStatus = info.LastStatus
					afile.StatusTime = info.StatusTime
					if strings.Contains(afile.LastStatus, "failed") {
						afile.StatusColor = "red"
					} else if strings.Contains(afile.LastStatus, "stop") ||
						strings.Contains(afile.LastStatus, "crash") {
						afile.StatusColor = "#aa4444"
					}
					if afile.StatusTime.After(time.Now().Add(time.Hour * -12)) {
						afile.TimeColor = "blue"
					} else if afile.StatusTime.After(time.Now().Add(time.Hour * -24)) {
						afile.TimeColor = "navy"
					}
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

	if !strings.Contains(jsonfilename, "/") {
		jsonfilename = getInfoFilename(jsonfilename)
	}
	success = false

	contents, err := ioutil.ReadFile(jsonfilename)

	if err != nil {

		return
	} else {
		err := json.Unmarshal(contents, &info)
		if err != nil {
			writeToLog("Error in readAppConfig: " + err.Error())
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
		if (strings.Contains(lines[i], appname)) && (!strings.Contains(lines[i], "grep")) {
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

		cmd.Run()
	}
	result = out.String()
	errorMsg = err.String()
	return
}

func getLinuxUser() string {
	result, err := runShell(Run, "/bin/sh", "-c", "whoami")

	if err != "" {
		writeToLog("Error: " + err)
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

		err = os.Remove(targetname)
		if err != nil {
			if strings.Contains(err.Error(), "no such file or directory") {
				err = nil
			}
		}
		if err == nil {
			target, err := os.Create(targetname)
			if err == nil {
				defer target.Close()
				_, err = io.Copy(target, source)
				if err == nil {
					CopyFileInfo(sourcename, targetname)
				}
			}
		}

	}
	return err
}

func runApp(appname string) (errorMsg string) {

	filename := getAppDir() + appname + "/start.sh"
	reWriteFile(filename, appname)
	var result string
	result, errorMsg = Shell(filename)
	writeToLog("Running " + appname + ": " + result + ": " + errorMsg)
	return
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

func checkClosedApps(startType string) {

	CloseMutex.Lock()
	defer CloseMutex.Unlock()
	list := getAppList()
	for _, item := range list {
		if item.IsRunning {

			isRunning, _ := isAppRunning(item.AppName)

			if !isRunning {
				_, details := readAppConfig(item.AppName)
				details.StatusTime = time.Now()
				details.LastStatus = startType
				runApp(item.AppName)
				isRunning, _ := isAppRunning(item.AppName)
				if !isRunning {
					details.LastStatus = "failed start"
					details.Counter++
					if details.Counter > 10 {
						details.IsRunning = false
					}
				}
				setConfigFile(details, item.AppName)
				writeToLog("Starting after close/crash: " + item.AppName)
			}

		}
	}
}

func check() {
	for {
		time.Sleep(time.Second * 10)
		checkClosedApps("crash start")
	}
}

func reWriteFile(filename, appname string) {

	file, err := os.OpenFile(filename, os.O_RDONLY, 0644)
	if err == nil {
		outfile, outerr := os.OpenFile(filename+".tmp", os.O_CREATE|os.O_WRONLY, 0644)
		defer file.Close()
		if outerr == nil {
			defer outfile.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, "&") && strings.Contains(line, appname) {
					if !strings.Contains(line, "nohup") {
						line = "nohup " + line
					}
					if !strings.Contains(line, ">") {
						indx := strings.Index(line, "&")
						line = line[:indx] + " >> log.out 2>&1  " + line[indx:]
					}
				}
				outfile.WriteString(line + "\n")
			}
			os.Remove(filename)
			os.Rename(filename+".tmp", filename)
		}
	}
	return
}

func CopyFileInfo(source, target string) (err error) {

	var fileInfo os.FileInfo
	fileInfo, err = os.Stat(source)
	if err == nil {

		err = os.Chtimes(target, time.Now(), fileInfo.ModTime())
		os.Chmod(target, fileInfo.Mode())
	}
	return
}

func ArchiveOldFile(appname, targetFilename string) (err error) {

	if codeutils.IsFileExists(targetFilename) {
		archivefolder := getAppDir() + appname + "/archivefiles"
		err = os.Mkdir(archivefolder, os.ModePerm)

		if err == nil {
			index := strings.LastIndex(targetFilename, "/")
			if index > 0 {
				filename := targetFilename[index+1:]
				err = copyFile(targetFilename, archivefolder+"/"+filename)
			}
		}
	}
	return
}
