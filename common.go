// common
package main

import (
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

func getConfigValue(valuename string, defaultvalue string) (value string) {

	value = codeutils.GetConfigValue("gocat.ini", valuename)
	if value == "" {
		value = defaultvalue
	}
	return
}

type AppInfo struct {
	Filename  string
	Port      string
	Running   string
	Address   string
	FileTime  string
	IsRunning bool
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
				fullfilename := dir + afile.Filename + "/" + afile.Filename
				//afile.FileTime = f.ModTime().String()
				hasJson := false
				fileInfo, err := os.Stat(fullfilename)
				port := ""
				if err == nil {

					afile.FileTime = fileInfo.ModTime().String()
					afile.FileTime = afile.FileTime[:19]
					hasJson, port = getPort(fullfilename + ".json")
					afile.Port = port
				}

				if hasJson {
					println(fullfilename, " has JSON")
					afile.Address = address + ":" + port
					afile.IsRunning = isAppRunning(afile.Filename)
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

func listShelfApplications(w http.ResponseWriter, r *http.Request) []string {

	dir := getAppDir() + "shelf.dir"
	files, err := ioutil.ReadDir(dir)
	var list []string
	if err == nil {

		for _, f := range files {
			if !f.IsDir() {
				list = append(list, f.Name())
			}

		}
	}

	return list

}

func listFiles(dir string, w http.ResponseWriter) []FileInfo {

	files, _ := ioutil.ReadDir(dir)

	var list []FileInfo
	for _, f := range files {

		var afile FileInfo
		afile.FileName = f.Name()

		afile.FileTime = f.ModTime().String()
		if f.Size() < 1024 {
			afile.Size = fmt.Sprintf("%d", f.Size())
		} else if f.Size() < 1024*1024 {
			afile.Size = fmt.Sprintf("%0.1f K", float64(f.Size())/1000)
		} else {
			afile.Size = fmt.Sprintf("%0.1f M", (float32(f.Size())/1000)/1000)

		}

		afile.IsDir = f.IsDir()

		ext := filepath.Ext(f.Name())
		afile.IsEdit = !afile.IsDir && ext != ""
		list = append(list, afile)

	}

	return list
}

func getPort(jsonfilename string) (success bool, port string) {

	success = false

	contents, err := ioutil.ReadFile(jsonfilename)
	if err != nil {
		println(err.Error())
		return
	} else {
		var info DetailFile
		err := json.Unmarshal(contents, &info)
		if err != nil {
			println("Error in getPort: ", err.Error())
		}
		port = info.Port
		success = true
		return
	}

}

func isAppRunning(appname string) bool {

	var out bytes.Buffer

	cmd := exec.Command("/bin/bash", "-c", "ps -ef | grep "+appname)
	cmd.Stdout = &out
	cmd.Run()

	exist := false

	lines := strings.Split(out.String(), "\n")
	for i := 0; i < len(lines); i++ {
		if (strings.Contains(lines[i], appname)) && (!strings.Contains(lines[i], "grep")) &&
			(!strings.Contains(lines[i], "check")) {
			exist = true
		}
	}

	return exist
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
		os.Remove(targetname)
		target, err := os.OpenFile(targetname, os.O_WRONLY|os.O_CREATE, 0766)
		if err == nil {
			_, err = io.Copy(target, source)
		}
		target.Close()
	}
	return err
}

func runApp(appname string) {
	filename := getAppDir() + appname + "/start.sh"
	Shell(filename) //; write result to log
}

func stopIfRunning(filename string, toShelf bool) (isAlreadyRunning bool) {

	isAlreadyRunning = isAppRunning(filename)
	if !toShelf && isAlreadyRunning {
		executeKill(filename)
		time.Sleep(time.Second * 4)
	}
	return
}

func writeToLog(event string) {
	codeutils.WriteToLog(event, "gocat")
}
