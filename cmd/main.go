package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
	// Create a global context with cancel function
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Go routine to handle shutdown signals
	go func() {
		sig := <-signalChan
		fmt.Printf("Received signal: %s, shutting down gracefully...\n", sig)
		cancel()
	}()

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

	mc := minio.MinioClient{}
	mc.InitMinio(endpoint, accessKeyID, secretAccessKey, useSSL)

	fileCache := memorycache.NewSafeCache(ctx)
	go fileCache.DeletingLoop(5)

	fileHandler := FileHandler{
		fileCache:   fileCache,
		minioClient: &mc,
	}

	// Server setup
	server := &http.Server{
		Addr: ":8080",
	}

	http.Handle("/get-file/{fileName}", &fileHandler)
	http.HandleFunc("/ping", ping)

	// Start server in a goroutine so it doesn't block
	go func() {
		fmt.Println("Server started on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %s\n", err)
		}
	}()

	// Wait for context cancellation to begin shutdown
	<-ctx.Done()

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Shutdown the server
	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("Server shutdown error: %s\n", err)
	} else {
		fmt.Println("Server gracefully stopped")
	}
}
