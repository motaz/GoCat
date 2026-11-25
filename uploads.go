package main

import (
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/motaz/codeutils"
)

type UploadedFileInfo struct {
	Filename string
	Message  string
	Success  bool
}

func getFileName(contentDisposition string) (filename string) {
	filename = contentDisposition[strings.Index(contentDisposition, "filename=")+10:]
	filename = strings.Trim(filename, "\"")
	return
}

func uploadfiles(w http.ResponseWriter, r *http.Request) []UploadedFileInfo {

	var result []UploadedFileInfo

	dir := r.FormValue("dir")
	validatedDir, err := secureJoin(getAppDir(), dir)
	if err != nil {
		return result
	}
	dir = validatedDir

	r.ParseMultipartForm(32 << 20)
	files := r.MultipartForm.File["file"]
	for _, onefile := range files {
		file, _ := onefile.Open()

		filename := filepath.Base(onefile.Filename)
		if filename == "" || filename == "." || filename == ".." {
			continue
		}

		afilename, err := secureJoin(dir, filename)
		if err != nil {
			continue
		}
		if strings.Contains(afilename, "/") {
			folder := afilename[0:strings.LastIndex(afilename, "/")]
			if !codeutils.IsFileExists(folder) {
				os.MkdirAll(folder, os.ModePerm)

			}
		}
		os.Remove(afilename)
		f, _ := os.OpenFile(afilename, os.O_WRONLY|os.O_CREATE, 0766)

		defer f.Close()
		_, err = io.Copy(f, file)
		var uploadedFile UploadedFileInfo
		uploadedFile.Filename = filename
		writeToLog("Uploading: " + afilename)
		if err == nil {
			uploadedFile.Message = "has been uploaded succesfully"
			uploadedFile.Success = true
		} else {
			uploadedFile.Message = "Error in upload: " + err.Error()
			uploadedFile.Success = false
		}
		writeToLog(uploadedFile.Message)
		result = append(result, uploadedFile)

	}
	return result

}

// Upload application
func upload(w http.ResponseWriter, r *http.Request, indexTemplate *IndexTemplate) {

	w.Header().Add("Content-Type", "text/html;charset=UTF-8")
	w.Header().Add("encoding", "UTF-8")

	toShelf := r.FormValue("shelf") == "1"
	r.ParseMultipartForm(32 << 20)
	file, handler, err := r.FormFile("file")
	if err != nil {
		indexTemplate.Message = "Error: " + err.Error()
		indexTemplate.Class = "errormessage"

	} else {
		defer file.Close()
		aname := filepath.Base(handler.Filename)
		writeToLog(aname)
		ex := filepath.Ext(aname)
		if ex == ".gz" || ex == ".zip" {
			aname = aname[:strings.Index(aname, ex)]
		}
		if !validAppName(aname) {
			indexTemplate.Message = "Error: invalid application name"
			indexTemplate.Class = "errormessage"
			return
		}
		isAlreadyRunning := stopIfRunning(aname, toShelf)
		// Put file in location
		var dir, adir string
		if toShelf {
			dir = getShelfDir()
			adir = dir
		} else {
			dir = getAppDir() + aname + "/"
			adir = getAppDir() + aname + "/"
		}

		if !codeutils.IsFileExists(adir) {
			os.MkdirAll(adir, os.ModePerm)
		}
		afilename := adir + handler.Filename
		writeToLog("Uploading application: " + afilename)

		// App Info
		infoFilename := adir + aname + ".json"
		port := r.FormValue("port")
		if port == "" {
			_, config := readAppConfig(infoFilename)
			port = config.Port
		}

		// Configuration
		if !toShelf {
			var details DetailFile
			details.Port = port
			details.AppName = aname
			details.Counter = 0
			details.StatusTime = time.Now()
			details.LastStatus = "Upload"
			err = writeConfigFile(details, infoFilename, *indexTemplate)

		}

		if err == nil {
			if !toShelf {
				if !validAppName(aname) {
					indexTemplate.Message = "Error: invalid application name"
					indexTemplate.Class = "errormessage"
					return
				}

				err = writeStartScript(adir, aname)
				ArchiveOldFile(aname, afilename)
			}

			copyAndPutFile(afilename, indexTemplate, file, handler.Filename,
				toShelf)

		}

		isRunning, _ := isAppRunning(aname)
		if !toShelf && isAlreadyRunning && !isRunning {
			runApp(aname)
		}

	}
}

func gunzipFile(filename string) error {
	input, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer input.Close()

	reader, err := gzip.NewReader(input)
	if err != nil {
		return err
	}
	defer reader.Close()

	destination := strings.TrimSuffix(filename, ".gz")
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(output, reader)
	if err == nil {
		err = os.Remove(filename)
	}
	return err
}

func unzipFile(filename, destinationDir string) error {
	reader, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}
	defer reader.Close()

	destinationRoot, err := filepath.Abs(destinationDir)
	if err != nil {
		return err
	}
	for _, entry := range reader.File {
		target, err := filepath.Abs(filepath.Join(destinationRoot, entry.Name))
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(destinationRoot, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("zip entry escapes destination: %s", entry.Name)
		}

		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, entry.Mode())
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return os.Remove(filename)
}

func writeStartScript(dir string, filename string) error {

	scriptFileName := dir + "start.sh"
	script := "#!/bin/bash\n" +
		"cd " + dir + "\n" +
		"nohup ./" + filename + " >> log.out 2>&1 &\n"
	err := writeToFile(scriptFileName, []byte(script))
	return err
}

func setConfigFile(details DetailFile, infoFilename string) error {

	if !strings.Contains(infoFilename, "/") {
		infoFilename = getInfoFilename(infoFilename)
	}
	jsonData, err := json.Marshal(details)

	err = writeToFile(infoFilename, jsonData)

	return err
}

func writeConfigFile(details DetailFile, infoFilename string, indexTemplate IndexTemplate) (err error) {

	err = setConfigFile(details, infoFilename)
	if err != nil {
		indexTemplate.Message = "Error: " + err.Error()
		indexTemplate.Class = "errormessage"

	}
	return err
}

func copyAndPutFile(fullfilename string, indexTemplate *IndexTemplate,
	file multipart.File, onlyfilename string, toShelf bool) {

	os.Remove(fullfilename + ".tmp")

	tempFile, err := os.OpenFile(fullfilename+".tmp", os.O_WRONLY|os.O_CREATE, os.ModePerm)
	if err != nil {
		indexTemplate.Message = err.Error()
		indexTemplate.Class = "errormessage"

	} else {
		// Copy application file to temp file
		_, err := io.Copy(tempFile, file)
		tempFile.Close()
		if err != nil {
			indexTemplate.Message = "copy error: " + err.Error()
			indexTemplate.Class = "errormessage"
		} else {
			// Copy to origional file
			err = copyFile(fullfilename+".tmp", fullfilename)

			if err == nil {
				ex := filepath.Ext(fullfilename)

				if ex == ".gz" || ex == ".zip" {
					if ex == ".gz" {
						err := os.Remove(fullfilename[:strings.Index(fullfilename, ex)])
						if err != nil && !strings.Contains(err.Error(), "no such file or directory") {
							writeToLog("Error remove old to uncompress: " + err.Error())
						}
						runErr := gunzipFile(fullfilename)
						if runErr != nil {
							indexTemplate.Message = "Error: " + runErr.Error()
							indexTemplate.Class = "errormessage"
						}
					} else if ex == ".zip" {
						err := os.Remove(fullfilename[:strings.Index(fullfilename, ex)])
						if err != nil && !strings.Contains(err.Error(), "no such file or directory") {
							writeToLog("Error remove old to uncompress: " + err.Error())
						}
						runErr := unzipFile(fullfilename, filepath.Dir(fullfilename))
						if runErr != nil {
							indexTemplate.Message = "Error: " + runErr.Error()
							indexTemplate.Class = "errormessage"
						}
					}
					if ex == ".zip" && indexTemplate.Class != "errormessage" {

						os.Remove(fullfilename)
					}

				}
				if indexTemplate.Class != "errormessage" {
					if toShelf {
						indexTemplate.Message = "File uploaded to shelf: " + onlyfilename
					} else {
						indexTemplate.Message = "File uploaded: " + onlyfilename
					}
					indexTemplate.Class = "infomessage"
				}
				os.Remove(fullfilename + ".tmp")
			} else {

				indexTemplate.Message = "Error uploading file: " + onlyfilename + ": " + err.Error()
				indexTemplate.Class = "errormessage"
				writeToLog(indexTemplate.Message)
			}
		}
	}
}
