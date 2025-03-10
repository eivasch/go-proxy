package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var MinioClient *minio.Client

func initMinio(endpoint, accessKey, secretKey string, useSSL bool) *minio.Client {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return client
}

func init() {
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

	MinioClient = initMinio(endpoint, accessKeyID, secretAccessKey, useSSL)
}

func GetMinioFile(fileName string) []byte {
	reader, err := MinioClient.GetObject(context.Background(), "my-first-bucket", fileName, minio.GetObjectOptions{})
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer reader.Close()
	fmt.Println("Successfully got the object")

	buffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(buffer, reader); err != nil {
		fmt.Println("Error reading file:", err)
		return nil
	}
	return buffer.Bytes()
}
