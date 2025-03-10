package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

)

var FileCache = make(map[string][]byte)

func ping(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Pong")
}

func getFile(w http.ResponseWriter, r *http.Request) {
	fileName := r.PathValue("fileName")
	fmt.Printf("Requested file name %s\n", fileName)

	if _, ok := FileCache[fileName]; ok {
		fmt.Println("File found in cache")
		w.Write(FileCache[fileName])
		return
	}
	fmt.Println("File not found in cache")

	file := GetMinioFile(fileName)
	FileCache[fileName] = file

	io.Copy(w, bytes.NewReader(FileCache[fileName]))
	w.Header().Set("Content-Type", "application/octet-stream")
}

func main() {
	http.HandleFunc("/ping", ping)
	http.HandleFunc("/get-file/{fileName}", getFile)
	http.ListenAndServe(":8080", nil)
}
