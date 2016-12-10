package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/encryptio/go-git-annex-external/external"
	"gopkg.in/kothar/go-backblaze.v0"
)

type B2Ext struct {
	bucket *backblaze.Bucket
	prefix string

	lastList struct {
		setAt time.Time
		file  string
		found bool
		id    string
	}
}

func authenticate(e *external.External) (*backblaze.B2, error) {
	accountID, err := e.GetConfig("accountid")
	if err != nil {
		return nil, err
	}
	if accountID == "" {
		accountID = os.Getenv("B2_ACCOUNT_ID")
	}
	if accountID == "" {
		return nil, errors.New("You must set accountid to the backblaze account id")
	}

	appKey, err := e.GetConfig("appkey")
	if err != nil {
		return nil, err
	}
	if appKey == "" {
		appKey = os.Getenv("B2_APP_KEY")
	}
	if appKey == "" {
		return nil, errors.New("You must set appkey to the backblaze application key")
	}

	b2, err := backblaze.NewB2(backblaze.Credentials{
		AccountID:      accountID,
		ApplicationKey: appKey,
	})
	if err != nil {
		return nil, fmt.Errorf("Couldn't authorize: %v", err)
	}

	return b2, nil
}

func getBucketConfig(e *external.External) (bucket string, prefix string, err error) {
	bucket, err = e.GetConfig("bucket")
	if err != nil {
		return "", "", err
	}
	if bucket == "" {
		return "", "", errors.New("You must set bucket to the bucket name")
	}

	prefix, err = e.GetConfig("prefix")
	// prefix == "" is ok.
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	return bucket, prefix, nil
}

func (be *B2Ext) listFileCached(file string) (found bool, fileID string, err error) {
	// Caching the last result of ListFileNames is no less safe than not caching
	// it; the race condition of two concurrent git annex copy --to b2 processes
	// sending the same file can result in a file with two identical versions in
	// both cases.
	//
	// However, caching this reduces the number of ListFileNames to half of what
	// it is during uploads (since git-annex always calls checkpresent which
	// uses ListFileNames before uploading, but when uploading we also do
	// upload elision by calling ListFileNames.)

	if be.lastList.file != file || time.Since(be.lastList.setAt) > time.Second*15 {
		res, err := be.bucket.ListFileNames(file, 1)
		if err != nil {
			return false, "", err
		}

		be.lastList.setAt = time.Now()
		if len(res.Files) == 0 || res.Files[0].Name != file {
			be.lastList.file = file
			be.lastList.found = false
			be.lastList.id = ""
		} else {
			be.lastList.file = file
			be.lastList.found = true
			be.lastList.id = res.Files[0].ID
		}
	}

	return be.lastList.found, be.lastList.id, nil
}

func (be *B2Ext) clearListFileCache() {
	be.lastList.setAt = time.Time{}
	be.lastList.file = ""
	be.lastList.found = false
	be.lastList.id = ""
}

func (be *B2Ext) setup(e *external.External, canCreateBucket bool) error {
	if be.bucket != nil {
		// already done!
		return nil
	}

	b2, err := authenticate(e)
	if err != nil {
		return err
	}

	bucketName, prefix, err := getBucketConfig(e)
	if err != nil {
		return err
	}

	bucket, err := b2.Bucket(bucketName)
	if err != nil {
		return fmt.Errorf("couldn't open bucket %#v: %v", bucketName, err)
	}

	if bucket == nil {
		if !canCreateBucket {
			return fmt.Errorf("bucket %#v does not exist anymore", bucketName)
		}

		fmt.Fprintf(os.Stderr, "Creating private B2 bucket %#v\n", bucketName)

		bucket, err = b2.CreateBucket(bucketName, backblaze.AllPrivate)
		if err != nil {
			return fmt.Errorf("couldn't create bucket %#v: %v", bucketName, err)
		}
	}

	be.bucket = bucket
	be.prefix = prefix

	return nil
}

func (be *B2Ext) InitRemote(e *external.External) error {
	return be.setup(e, true)
}

func (be *B2Ext) Prepare(e *external.External) error {
	return be.setup(e, false)
}

func (be *B2Ext) Store(e *external.External, key, file string) error {
	fh, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fh.Close()

	shaReady := make(chan struct{})
	var haveSHA []byte
	var contentLength int64
	var shaError error
	go func() {
		defer close(shaReady)

		sha := sha1.New()
		contentLength, shaError = io.Copy(sha, fh)
		if shaError != nil {
			return
		}

		haveSHA = sha.Sum(nil)

		_, shaError = fh.Seek(0, 0)
	}()

	found, fileID, err := be.listFileCached(be.prefix + key)
	if err != nil {
		return fmt.Errorf("couldn't list filenames: %v", err)
	}

	if found {
		// file probably already stored; make sure using the SHA1
		b2file, err := be.bucket.GetFileInfo(fileID)
		if err != nil {
			return fmt.Errorf("couldn't get file info for %#v: %v", fileID, err)
		}
		if b2file != nil {
			<-shaReady

			wantSHA, err := hex.DecodeString(b2file.ContentSha1)
			if err == nil && bytes.Equal(haveSHA, wantSHA) {
				// File already exists with correct data.
				return nil
			}

			// File exists but is the incorrect data. Delete the old version
			// first; B2 will keep the old version around otherwise.
			_, err = be.bucket.DeleteFileVersion(be.prefix+key, b2file.ID)
			if err != nil {
				return fmt.Errorf("couldn't delete old file version: %v", err)
			}
		}
	}

	<-shaReady
	if shaError != nil {
		return fmt.Errorf("couldn't hash local file %v: %v", file, shaError)
	}

	_, err = be.bucket.UploadHashedFile(
		be.prefix+key,
		nil,
		external.NewProgressReader(fh, e),
		hex.EncodeToString(haveSHA),
		contentLength)

	be.clearListFileCache()

	if err != nil {
		return fmt.Errorf("couldn't upload file: %v", err)
	}

	return nil
}

func (be *B2Ext) Retrieve(e *external.External, key, file string) error {
	fh, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("couldn't open %v for writing: %v", file, err)
	}
	defer fh.Close()

	_, rc, err := be.bucket.DownloadFileByName(be.prefix + key)
	if rc != nil {
		defer rc.Close()
	}
	if err != nil {
		return err
	}

	_, err = io.Copy(fh, external.NewProgressReader(rc, e))
	if err != nil {
		return err
	}

	return nil
}

func (be *B2Ext) CheckPresent(e *external.External, key string) (bool, error) {
	found, _, err := be.listFileCached(be.prefix + key)
	if err != nil {
		return false, fmt.Errorf("couldn't list filenames: %v", err)
	}

	return found, nil
}

func (be *B2Ext) Remove(e *external.External, key string) error {
	found, fileID, err := be.listFileCached(be.prefix + key)
	if err != nil {
		return fmt.Errorf("couldn't list filenames: %v", err)
	}

	if !found {
		// File already non-existent, nothing to remove
		return nil
	}

	_, err = be.bucket.DeleteFileVersion(be.prefix+key, fileID)
	be.clearListFileCache()
	if err != nil {
		return fmt.Errorf("couldn't delete file version: %v", err)
	}

	return nil
}

func (be *B2Ext) GetCost(e *external.External) (int, error) {
	return 0, external.ErrUnsupportedRequest
}

func (be *B2Ext) GetAvailability(e *external.External) (external.Availability, error) {
	return external.AvailabilityGlobal, nil
}

func (be *B2Ext) WhereIs(e *external.External, key string) (string, error) {
	return "", external.ErrUnsupportedRequest
}

func main() {
	h := &B2Ext{}

	var (
		in  io.Reader = os.Stdin
		out io.Writer = os.Stdout
	)

	if os.Getenv("GIT_ANNEX_EXTERNAL_B2_PROTOCOL_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "git-annex-remote-b2: enabling protocol debug logging\n")
		in = io.TeeReader(in, os.Stderr)
		out = io.MultiWriter(out, os.Stderr)
	}

	err := external.RunLoop(in, out, h)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}
