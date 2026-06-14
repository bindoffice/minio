/*
 * MinIO Cloud Storage, (C) 2026 bindoffice
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// migrate-from-2024 copies buckets and objects from a newer MinIO cluster
// (e.g. 2024 release) into this legacy-compatible deployment.
//
// Usage (from repository root):
//
//	go run ./docs/migration/migrate-from-2024/ \
//	  -source-endpoint minio2024.example.com:9000 \
//	  -source-access-key SOURCEKEY -source-secret-key SOURCESECRET \
//	  -dest-endpoint minio-legacy.example.com:9000 \
//	  -dest-access-key DESTKEY -dest-secret-key DESTSECRET \
//	  -dry-run
//
// What is migrated:
//   - Buckets (created on destination when missing)
//   - Bucket IAM policy JSON (when readable from source)
//   - Object data, content-type, and user metadata (latest version only)
//
// What is NOT migrated (configure manually on destination if needed):
//   - Bucket replication, lifecycle (ILM), notification, encryption config
//   - Object lock / retention / legal hold
//   - Object version history (use `mc mirror --version` instead)
//   - STS / IAM users (use mc admin or madmin separately)
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type config struct {
	sourceEndpoint string
	sourceAccess   string
	sourceSecret   string
	sourceSecure   bool
	sourceInsecure bool

	destEndpoint string
	destAccess   string
	destSecret   string
	destSecure   bool
	destInsecure bool

	region       string
	buckets      string
	prefix       string
	workers      int
	dryRun       bool
	skipExisting bool
}

type objectTask struct {
	bucket string
	key    string
}

type stats struct {
	bucketsCreated int64
	objectsListed  int64
	objectsCopied  int64
	objectsSkipped int64
	objectsFailed  int64
	bytesCopied    int64
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg := config{}
	flag.StringVar(&cfg.sourceEndpoint, "source-endpoint", "", "Source MinIO endpoint host:port (2024 cluster)")
	flag.StringVar(&cfg.sourceAccess, "source-access-key", "", "Source access key")
	flag.StringVar(&cfg.sourceSecret, "source-secret-key", "", "Source secret key")
	flag.BoolVar(&cfg.sourceSecure, "source-secure", true, "Use HTTPS for source")
	flag.BoolVar(&cfg.sourceInsecure, "source-insecure", false, "Skip TLS verification for source")

	flag.StringVar(&cfg.destEndpoint, "dest-endpoint", "", "Destination MinIO endpoint host:port (legacy cluster)")
	flag.StringVar(&cfg.destAccess, "dest-access-key", "", "Destination access key")
	flag.StringVar(&cfg.destSecret, "dest-secret-key", "", "Destination secret key")
	flag.BoolVar(&cfg.destSecure, "dest-secure", true, "Use HTTPS for destination")
	flag.BoolVar(&cfg.destInsecure, "dest-insecure", false, "Skip TLS verification for destination")

	flag.StringVar(&cfg.region, "region", "us-east-1", "Region for newly created buckets")
	flag.StringVar(&cfg.buckets, "buckets", "", "Comma-separated bucket list (default: all buckets on source)")
	flag.StringVar(&cfg.prefix, "prefix", "", "Only migrate objects with this key prefix")
	flag.IntVar(&cfg.workers, "workers", 8, "Concurrent object copy workers")
	flag.BoolVar(&cfg.dryRun, "dry-run", false, "List actions without writing to destination")
	flag.BoolVar(&cfg.skipExisting, "skip-existing", true, "Skip when destination object etag matches source")
	flag.Parse()

	if cfg.sourceEndpoint == "" || cfg.destEndpoint == "" {
		flag.Usage()
		log.Fatal("source-endpoint and dest-endpoint are required")
	}
	if cfg.sourceAccess == "" || cfg.sourceSecret == "" || cfg.destAccess == "" || cfg.destSecret == "" {
		flag.Usage()
		log.Fatal("source and destination credentials are required")
	}
	if cfg.workers < 1 {
		log.Fatal("workers must be >= 1")
	}

	ctx := context.Background()

	src, err := newClient(cfg.sourceEndpoint, cfg.sourceAccess, cfg.sourceSecret, cfg.sourceSecure, cfg.sourceInsecure)
	if err != nil {
		log.Fatalf("source client: %v", err)
	}
	dst, err := newClient(cfg.destEndpoint, cfg.destAccess, cfg.destSecret, cfg.destSecure, cfg.destInsecure)
	if err != nil {
		log.Fatalf("dest client: %v", err)
	}

	bucketNames, err := resolveBuckets(ctx, src, cfg.buckets)
	if err != nil {
		log.Fatalf("list buckets: %v", err)
	}
	if len(bucketNames) == 0 {
		log.Fatal("no buckets to migrate")
	}

	var st stats
	log.Printf("migrating %d bucket(s) from %s -> %s (dry-run=%v workers=%d)",
		len(bucketNames), cfg.sourceEndpoint, cfg.destEndpoint, cfg.dryRun, cfg.workers)

	for _, bucket := range bucketNames {
		if err := ensureBucket(ctx, src, dst, bucket, cfg, &st); err != nil {
			log.Fatalf("bucket %q setup: %v", bucket, err)
		}
		if err := migrateBucketPolicy(ctx, src, dst, bucket, cfg.dryRun); err != nil {
			log.Printf("bucket %q policy: %v (continuing)", bucket, err)
		}
		if err := migrateObjects(ctx, src, dst, bucket, cfg, &st); err != nil {
			log.Fatalf("bucket %q objects: %v", bucket, err)
		}
	}

	log.Printf("done: buckets_created=%d listed=%d copied=%d skipped=%d failed=%d bytes=%d",
		st.bucketsCreated, st.objectsListed, st.objectsCopied, st.objectsSkipped, st.objectsFailed, st.bytesCopied)
	if st.objectsFailed > 0 {
		log.Fatal("migration finished with errors")
	}
}

func newClient(endpoint, accessKey, secretKey string, secure, insecureTLS bool) (*minio.Client, error) {
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	}
	if insecureTLS {
		opts.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // operator opt-in
		}
	}
	return minio.New(endpoint, opts)
}

func resolveBuckets(ctx context.Context, src *minio.Client, bucketsFlag string) ([]string, error) {
	if bucketsFlag != "" {
		parts := strings.Split(bucketsFlag, ",")
		out := make([]string, 0, len(parts))
		for _, b := range parts {
			b = strings.TrimSpace(b)
			if b != "" {
				out = append(out, b)
			}
		}
		return out, nil
	}
	names := make([]string, 0, 32)
	buckets, err := src.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	for _, bucket := range buckets {
		names = append(names, bucket.Name)
	}
	return names, nil
}

func ensureBucket(ctx context.Context, src, dst *minio.Client, bucket string, cfg config, st *stats) error {
	exists, err := dst.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if exists {
		log.Printf("bucket %q already exists on destination", bucket)
		return nil
	}

	// Probe source bucket exists.
	exists, err = src.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("source bucket missing")
	}

	if cfg.dryRun {
		log.Printf("[dry-run] would create bucket %q", bucket)
		atomic.AddInt64(&st.bucketsCreated, 1)
		return nil
	}

	if err := dst.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: cfg.region}); err != nil {
		return err
	}
	atomic.AddInt64(&st.bucketsCreated, 1)
	log.Printf("created bucket %q on destination", bucket)
	return nil
}

func migrateBucketPolicy(ctx context.Context, src, dst *minio.Client, bucket string, dryRun bool) error {
	policy, err := src.GetBucketPolicy(ctx, bucket)
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return err
	}
	if dryRun {
		log.Printf("[dry-run] would set policy on bucket %q (%d bytes)", bucket, len(policy))
		return nil
	}
	return dst.SetBucketPolicy(ctx, bucket, policy)
}

func migrateObjects(ctx context.Context, src, dst *minio.Client, bucket string, cfg config, st *stats) error {
	tasks := make(chan objectTask, cfg.workers*2)
	errCh := make(chan error, cfg.workers)

	var wg sync.WaitGroup
	for i := 0; i < cfg.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				if err := copyObject(ctx, src, dst, task, cfg, st); err != nil {
					atomic.AddInt64(&st.objectsFailed, 1)
					log.Printf("copy failed bucket=%q key=%q: %v", task.bucket, task.key, err)
					select {
					case errCh <- err:
					default:
					}
				}
			}
		}()
	}

	go func() {
		defer close(tasks)
		listLatest(ctx, src, bucket, cfg, tasks, st)
	}()

	wg.Wait()
	close(errCh)

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func listLatest(ctx context.Context, src *minio.Client, bucket string, cfg config, tasks chan<- objectTask, st *stats) {
	opts := minio.ListObjectsOptions{
		Recursive: true,
		Prefix:    cfg.prefix,
	}
	for obj := range src.ListObjects(ctx, bucket, opts) {
		if obj.Err != nil {
			log.Printf("list bucket=%q: %v", bucket, obj.Err)
			continue
		}
		atomic.AddInt64(&st.objectsListed, 1)
		tasks <- objectTask{bucket: bucket, key: obj.Key}
	}
}

func copyObject(ctx context.Context, src, dst *minio.Client, task objectTask, cfg config, st *stats) error {
	info, err := src.StatObject(ctx, task.bucket, task.key, minio.StatObjectOptions{})
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}

	if cfg.skipExisting {
		destInfo, err := dst.StatObject(ctx, task.bucket, task.key, minio.StatObjectOptions{})
		if err == nil && destInfo.ETag == info.ETag {
			atomic.AddInt64(&st.objectsSkipped, 1)
			return nil
		}
	}

	label := fmt.Sprintf("bucket=%q key=%q", task.bucket, task.key)

	if cfg.dryRun {
		log.Printf("[dry-run] would copy %s size=%d etag=%s", label, info.Size, info.ETag)
		atomic.AddInt64(&st.objectsCopied, 1)
		atomic.AddInt64(&st.bytesCopied, info.Size)
		return nil
	}

	reader, err := src.GetObject(ctx, task.bucket, task.key, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}
	defer reader.Close()

	putOpts := minio.PutObjectOptions{
		ContentType:  info.ContentType,
		UserMetadata: cloneMetadata(info.UserMetadata),
	}
	if enc := info.Metadata.Get("Content-Encoding"); enc != "" {
		putOpts.ContentEncoding = enc
	}

	start := time.Now()
	_, err = dst.PutObject(ctx, task.bucket, task.key, reader, info.Size, putOpts)
	if err != nil {
		return fmt.Errorf("put: %w", err)
	}
	// Drain reader in case PutObject did not consume all bytes.
	_, _ = io.Copy(io.Discard, reader)

	atomic.AddInt64(&st.objectsCopied, 1)
	atomic.AddInt64(&st.bytesCopied, info.Size)
	log.Printf("copied %s size=%d etag=%s in %s", label, info.Size, info.ETag, time.Since(start).Round(time.Millisecond))
	return nil
}

func cloneMetadata(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func isNotFound(err error) bool {
	var resp minio.ErrorResponse
	if errors.As(err, &resp) {
		return resp.Code == "NoSuchBucketPolicy" || resp.Code == "NoSuchKey" || resp.StatusCode == http.StatusNotFound
	}
	return false
}
