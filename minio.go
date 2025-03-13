package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
)

type MinioClient struct {
	client *minio.Client
}

func (mc *MinioClient) initMinio(endpoint, accessKey, secretKey string, useSSL bool) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	mc.client = client
}

func (mc *MinioClient) GetMinioFile(fileName string) ([]byte, error) {
	reader, err := mc.client.GetObject(context.Background(), "my-first-bucket", fileName, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	fmt.Println("Successfully got the object")

	buffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(buffer, reader); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
