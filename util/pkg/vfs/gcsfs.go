package vfs

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/api/googleapi"
	storage "google.golang.org/api/storage/v1"
	"io/ioutil"
	"k8s.io/kops/util/pkg/hashing"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
)

type GCSPath struct {
	client  *storage.Service
	bucket  string
	key     string
	md5Hash string
}

var _ Path = &GCSPath{}
var _ HasHash = &GCSPath{}

func NewGCSPath(client *storage.Service, bucket string, key string) *GCSPath {
	bucket = strings.TrimSuffix(bucket, "/")
	key = strings.TrimPrefix(key, "/")

	return &GCSPath{
		client: client,
		bucket: bucket,
		key:    key,
	}
}

func (p *GCSPath) Path() string {
	return "gs://" + p.bucket + "/" + p.key
}

func (p *GCSPath) Bucket() string {
	return p.bucket
}

func (p *GCSPath) String() string {
	return p.Path()
}

func (p *GCSPath) Remove() error {
	err := p.client.Objects.Delete(p.bucket, p.key).Do()
	if err != nil {
		// TODO: Check for not-exists, return os.NotExist

		return fmt.Errorf("error deleting %s: %v", p, err)
	}

	return nil
}

func (p *GCSPath) Join(relativePath ...string) Path {
	args := []string{p.key}
	args = append(args, relativePath...)
	joined := path.Join(args...)
	return &GCSPath{
		client: p.client,
		bucket: p.bucket,
		key:    joined,
	}
}

func (p *GCSPath) WriteFile(data []byte) error {
	glog.V(4).Infof("Writing file %q", p)

	md5Hash, err := hashing.HashAlgorithmMD5.Hash(bytes.NewReader(data))
	if err != nil {
		return err
	}

	obj := &storage.Object{
		Name:    p.key,
		Md5Hash: base64.StdEncoding.EncodeToString(md5Hash.HashValue),
	}
	r := bytes.NewReader(data)
	_, err = p.client.Objects.Insert(p.bucket, obj).Media(r).Do()
	if err != nil {
		return fmt.Errorf("error writing %s: %v", p, err)
	}

	return nil
}

// To prevent concurrent creates on the same file while maintaining atomicity of writes,
// we take a process-wide lock during the operation.
// Not a great approach, but fine for a single process (with low concurrency)
// TODO: should we enable versioning?
var createFileLockGCS sync.Mutex

func (p *GCSPath) CreateFile(data []byte) error {
	createFileLockGCS.Lock()
	defer createFileLockGCS.Unlock()

	// Check if exists
	_, err := p.ReadFile()
	if err == nil {
		return os.ErrExist
	}

	if !os.IsNotExist(err) {
		return err
	}

	return p.WriteFile(data)
}

func (p *GCSPath) ReadFile() ([]byte, error) {
	glog.V(4).Infof("Reading file %q", p)

	response, err := p.client.Objects.Get(p.bucket, p.key).Download()
	if err != nil {
		if isGCSNotFound(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("error reading %s: %v", p, err)
	}
	if response == nil {
		return nil, fmt.Errorf("no response returned from: %v", p, err)
	}
	defer response.Body.Close()

	d, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %v", p, err)
	}
	return d, nil
}

func (p *GCSPath) ReadDir() ([]Path, error) {
	prefix := p.key
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	ctx := context.Background()
	var paths []Path
	err := p.client.Objects.List(p.bucket).Delimiter("/").Prefix(prefix).Pages(ctx, func(page *storage.Objects) error {
		for _, o := range page.Items {
			child := &GCSPath{
				client:  p.client,
				bucket:  p.bucket,
				key:     o.Name,
				md5Hash: o.Md5Hash,
			}
			paths = append(paths, child)
		}
		return nil
	})
	if err != nil {
		if isGCSNotFound(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("error listing %s: %v", p, err)
	}
	glog.V(8).Infof("Listed files in %v: %v", p, paths)
	return paths, nil
}

func (p *GCSPath) ReadTree() ([]Path, error) {
	// No delimiter for recursive search

	prefix := p.key
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	ctx := context.Background()
	var paths []Path
	err := p.client.Objects.List(p.bucket).Prefix(prefix).Pages(ctx, func(page *storage.Objects) error {
		for _, o := range page.Items {
			key := o.Name
			child := &GCSPath{
				client:  p.client,
				bucket:  p.bucket,
				key:     key,
				md5Hash: o.Md5Hash,
			}
			paths = append(paths, child)
		}
		return nil
	})
	if err != nil {
		if isGCSNotFound(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("error listing %s: %v", p, err)
	}
	return paths, nil
}

func (p *GCSPath) Base() string {
	return path.Base(p.key)
}

func (p *GCSPath) PreferredHash() (*hashing.Hash, error) {
	return p.Hash(hashing.HashAlgorithmMD5)
}

func (p *GCSPath) Hash(a hashing.HashAlgorithm) (*hashing.Hash, error) {
	if a != hashing.HashAlgorithmMD5 {
		return nil, nil
	}

	md5 := p.md5Hash
	if md5 == "" {
		return nil, nil
	}

	md5Bytes, err := hex.DecodeString(md5)
	if err != nil {
		return nil, fmt.Errorf("Etag was not a valid MD5 sum: %q", md5)
	}

	return &hashing.Hash{Algorithm: hashing.HashAlgorithmMD5, HashValue: md5Bytes}, nil
}

func isGCSNotFound(err error) bool {
	if err == nil {
		return false
	}
	ae, ok := err.(*googleapi.Error)
	return ok && ae.Code == http.StatusNotFound
}