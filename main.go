package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"

	"github.com/joho/godotenv"
)

func ping(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Pong")
}

type FileHandler struct {
	fileCache   *SafeCache
	minioClient *MinioClient
}

func (fh *FileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	fileName := path.Clean(r.PathValue("fileName"))
	fmt.Printf("Requested file name %s\n", fileName)

	if file, ok := fh.fileCache.Get(fileName); ok {
		fmt.Println("File found in cache")
		w.Write(file)
		return
	}
	fmt.Println("File not found in cache")

	file, err := fh.minioClient.GetMinioFile(fileName)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	fh.fileCache.Set(fileName, file)

	io.Copy(w, bytes.NewReader(file))
	w.Header().Set("Content-Type", "application/octet-stream")
}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Warning: Error loading .env file:", err)
	}
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = "127.0.0.1:9000"
	}

	accessKeyID := os.Getenv("MINIO_ACCESS_KEY")
	secretAccessKey := os.Getenv("MINIO_SECRET_KEY")

	useSSL, err := strconv.ParseBool(os.Getenv("MINIO_USE_SSL"))
	if err != nil {
		useSSL = false
	}

	mc := MinioClient{}
	mc.initMinio(endpoint, accessKeyID, secretAccessKey, useSSL)

	fileCache := SafeCache{
		cache: make(map[string][]byte),
	}

	fileHandler := FileHandler{
		fileCache:   &fileCache,
		minioClient: &mc,
	}
	http.Handle("/get-file/{fileName}", &fileHandler)
	http.HandleFunc("/ping", ping)
	http.ListenAndServe(":8080", nil)
}
