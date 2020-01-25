package main

import (
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"github.com/motaz/codeutils"
)

type UploadedFileInfo struct {
	Filename string
	Message  string
	Success  bool
}

func uploadfiles(w http.ResponseWriter, r *http.Request) []UploadedFileInfo {

	var result []UploadedFileInfo

	dir := r.FormValue("dir")
	r.ParseMultipartForm(32 << 20)
	files := r.MultipartForm.File["file"]
	for _, onefile := range files {
		file, _ := onefile.Open()
		filename := onefile.Filename
		afilename := dir + "/" + filename
		if strings.Contains(afilename, "/") {
			folder := afilename[0:strings.LastIndex(afilename, "/")]
			if !codeutils.IsFileExists(folder) {
				os.MkdirAll(folder, os.ModePerm)

			}
		}
		os.Remove(afilename)
		f, _ := os.OpenFile(afilename, os.O_WRONLY|os.O_CREATE, 0766)

		defer f.Close()
		_, err := io.Copy(f, file)
		var uploadedFile UploadedFileInfo
		uploadedFile.Filename = filename

		if err == nil {
			uploadedFile.Message = "has been uploaded succesfully"
			uploadedFile.Success = true
		} else {
			uploadedFile.Message = "Error in upload: " + err.Error()
			uploadedFile.Success = false
		}

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
	isAlreadyRunning := stopIfRunning(handler.Filename, toShelf)
	defer file.Close()
	if err != nil {
		indexTemplate.Message = "Error: " + err.Error()
		indexTemplate.Class = "errormessage"

	} else {

		// Put file in location
		var dir string
		if toShelf {
			dir = getAppDir() + "shelf.dir/"
		} else {
			dir = getAppDir() + handler.Filename + "/"
		}
		if !codeutils.IsFileExists(dir) {
			os.MkdirAll(dir, os.ModePerm)
		}
		afilename := dir + handler.Filename
		{
			// App Info
			infoFilename := afilename + ".json"
			port := r.FormValue("port")
			if port == "" {
				_, port = getPort(infoFilename)
			}

			// Configuration
			if !toShelf {
				err = writeConfigFile(port, infoFilename, *indexTemplate)

			}

			if err == nil {
				if !toShelf {

					err = writeStartScript(dir, handler.Filename)
				}

				copyAndPutFile(afilename, indexTemplate, file, handler.Filename,
					toShelf)

			}

			if !toShelf && isAlreadyRunning && !isAppRunning(handler.Filename) {
				runApp(handler.Filename)
			}
		}

	}
}

func writeStartScript(dir string, filename string) error {
	scriptFileName := dir + "start.sh"
	script := "#!/bin/bash\n" +
		"cd " + dir + "\n" +
		"./" + filename + "&\n"
	err := writeToFile(scriptFileName, script)
	return err
}

func writeConfigFile(port string, infoFilename string, indexTemplate IndexTemplate) error {

	details := DetailFile{port}
	jsonData, err := json.Marshal(details)

	err = writeToFile(infoFilename, string(jsonData))
	if err != nil {
		indexTemplate.Message = "Error: " + err.Error()
		indexTemplate.Class = "errormessage"

	}
	return err
}

func copyAndPutFile(fullfilename string, indexTemplate *IndexTemplate,
	file multipart.File, onlyfilename string, toShelf bool) {

	os.Remove(fullfilename + ".tmp")
	tempFile, err := os.OpenFile(fullfilename+".tmp", os.O_WRONLY|os.O_CREATE, 777)
	if err != nil {
		indexTemplate.Message = err.Error()
		indexTemplate.Class = "errormessage"

	} else {
		// Copy application file to temp file
		_, err := io.Copy(tempFile, file)
		tempFile.Close()

		// Copy to origional file
		err = copyFile(fullfilename+".tmp", fullfilename)

		if err == nil {
			if toShelf {
				indexTemplate.Message = "File uploaded to shelf: " + onlyfilename
			} else {
				indexTemplate.Message = "File uploaded: " + onlyfilename
			}
			indexTemplate.Class = "infomessage"
			os.Remove(fullfilename + ".tmp")
		} else {

			indexTemplate.Message = "Error uploading file: " + onlyfilename + ": " + err.Error()
			indexTemplate.Class = "errormessage"
		}
	}
}
