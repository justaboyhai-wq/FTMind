// oss-upload-tree copies a local directory tree to an Alibaba Cloud OSS bucket.
//
// It is intended for one-off legacy storage migrations. Credentials are read
// from environment variables and are never printed. Existing objects are left
// unchanged unless -overwrite is explicitly supplied.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

type config struct {
	source    string
	prefix    string
	overwrite bool
	dryRun    bool
	endpoint  string
	region    string
	bucket    string
	accessKey string
	secretKey string
}

func main() {
	cfg := config{}
	flag.StringVar(&cfg.source, "source", "", "local directory to upload (required)")
	flag.StringVar(&cfg.prefix, "prefix", "", "destination key prefix (required)")
	flag.BoolVar(&cfg.overwrite, "overwrite", false, "overwrite objects that already exist")
	flag.BoolVar(&cfg.dryRun, "dry-run", false, "list planned uploads without writing objects")
	flag.Parse()

	cfg.endpoint = requiredEnv("OSS_ENDPOINT")
	cfg.region = requiredEnv("OSS_REGION")
	cfg.bucket = requiredEnv("OSS_BUCKET_NAME")
	cfg.accessKey = requiredEnv("OSS_ACCESS_KEY")
	cfg.secretKey = requiredEnv("OSS_SECRET_KEY")
	if cfg.source == "" || cfg.prefix == "" {
		fatal("-source and -prefix are required")
	}
	info, err := os.Stat(cfg.source)
	if err != nil || !info.IsDir() {
		fatal("source must be a readable directory")
	}

	creds := credentials.NewStaticCredentialsProvider(cfg.accessKey, cfg.secretKey, "")
	client := oss.NewClient(oss.LoadDefaultConfig().WithCredentialsProvider(creds).WithRegion(cfg.region).WithEndpoint(cfg.endpoint))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	exists, err := client.IsBucketExist(ctx, cfg.bucket)
	if err != nil {
		fatal("check bucket: " + err.Error())
	}
	if !exists {
		fatal("bucket does not exist or is not accessible")
	}

	prefix := strings.Trim(strings.ReplaceAll(cfg.prefix, "\\", "/"), "/")
	var uploaded, skipped int
	var totalBytes int64
	err = filepath.WalkDir(cfg.source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(cfg.source, path)
		if err != nil {
			return err
		}
		key := prefix + "/" + strings.ReplaceAll(filepath.ToSlash(rel), "\\", "/")
		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if cfg.dryRun {
			fmt.Printf("PLAN %s (%d bytes)\n", key, fileInfo.Size())
			uploaded++
			totalBytes += fileInfo.Size()
			return nil
		}
		if !cfg.overwrite {
			_, err = client.HeadObject(ctx, &oss.HeadObjectRequest{Bucket: oss.Ptr(cfg.bucket), Key: oss.Ptr(key)})
			if err == nil {
				fmt.Printf("SKIP %s\n", key)
				skipped++
				return nil
			}
			var serviceErr *oss.ServiceError
			if !errors.As(err, &serviceErr) || serviceErr.StatusCode != 404 {
				return fmt.Errorf("check existing object %s: %w", key, err)
			}
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
		request := &oss.PutObjectRequest{Bucket: oss.Ptr(cfg.bucket), Key: oss.Ptr(key), Body: io.Reader(file)}
		if contentType != "" {
			request.ContentType = oss.Ptr(contentType)
		}
		if _, err = client.PutObject(ctx, request); err != nil {
			return fmt.Errorf("upload %s: %w", key, err)
		}
		fmt.Printf("UPLOAD %s (%d bytes)\n", key, fileInfo.Size())
		uploaded++
		totalBytes += fileInfo.Size()
		return nil
	})
	if err != nil {
		fatal(err.Error())
	}
	mode := "uploaded"
	if cfg.dryRun {
		mode = "planned"
	}
	fmt.Printf("DONE %s=%d skipped=%d bytes=%d\n", mode, uploaded, skipped, totalBytes)
}

func requiredEnv(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		fatal("missing " + name)
	}
	return value
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "ERROR:", message)
	os.Exit(1)
}
