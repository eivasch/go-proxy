package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strconv"
	"syscall"
	"time"

	"proxy/pkg/memorycache"
	"proxy/pkg/minio"

	"github.com/joho/godotenv"
)

func ping(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Pong")
}

type FileHandler struct {
	fileCache   *memorycache.SafeCache
	minioClient *minio.MinioClient
}

func (fh *FileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	fileName := path.Clean(r.PathValue("fileName"))
	log.Printf("Requested file name %s\n", fileName)

	if file, ok := fh.fileCache.Get(fileName); ok {
		log.Println("File found in cache")
		w.Write(file)
		return
	}
	log.Println("File not found in cache")

	file, err := fh.minioClient.GetMinioFile(fileName)
	if err != nil {
		log.Println(err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	fh.fileCache.Set(fileName, file)

	io.Copy(w, bytes.NewReader(file))
	w.Header().Set("Content-Type", "application/octet-stream")
}

func main() {
	// Create a global context with cancel function
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Go routine to handle shutdown signals
	go func() {
		<-signalContext.Done()
		log.Printf("Received shutdown signal, canceling context...\n")
		cancel()
	}()

	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: Error loading .env file:", err)
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

	mc := minio.MinioClient{}
	mc.InitMinio(endpoint, accessKeyID, secretAccessKey, useSSL)

	fileCache := memorycache.NewSafeCache()
	go fileCache.DeletingLoop(5, ctx)

	fileHandler := FileHandler{
		fileCache:   fileCache,
		minioClient: &mc,
	}

	server := &http.Server{
		Addr: ":8080",
	}

	http.Handle("/get-file/{fileName}", &fileHandler)
	http.HandleFunc("/ping", ping)

	go func() {
		log.Println("Server started on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %s\n", err)
		}
	}()

	// Wait for context cancellation to begin shutdown
	<-ctx.Done()

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %s\n", err)
		return
	}
	log.Println("Server gracefully stopped")

}
