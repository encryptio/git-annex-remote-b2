package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"gopkg.in/kothar/go-backblaze.v0"
)

var (
	in     = bufio.NewReader(os.Stdin)
	out    = bufio.NewWriter(os.Stdout)
	bucket *backblaze.Bucket
	prefix string
)

func getConfig(name string) string {
	out.WriteString("GETCONFIG ")
	out.WriteString(name)
	out.WriteString("\n")
	out.Flush()

	line, err := in.ReadString('\n')
	if err != nil {
		log.Fatalf("Couldn't get config variable %s: %v", name, err)
	}
	line = strings.TrimSuffix(line, "\n")

	if strings.HasPrefix(line, "VALUE ") {
		return strings.TrimPrefix(line, "VALUE ")
	}

	return ""
}

func prepare(mode string) {
	err := prepareInternal()
	if err != nil {
		out.WriteString(mode + "-FAILURE " + err.Error() + "\n")
		return
	}

	out.WriteString(mode + "-SUCCESS\n")
}

func prepareInternal() error {
	accountID := getConfig("accountid")
	if accountID == "" {
		accountID = os.Getenv("B2_ACCOUNT_ID")
	}
	if accountID == "" {
		return errors.New("You must set accountid to the backblaze account id")
	}

	appKey := getConfig("appkey")
	if appKey == "" {
		appKey = os.Getenv("B2_APP_KEY")
	}
	if appKey == "" {
		return errors.New("You must set appkey to the backblaze application key")
	}

	bucketName := getConfig("bucket")
	if bucketName == "" {
		return errors.New("You must set bucket to the bucket name")
	}

	prefix = getConfig("prefix")
	// prefix == "" is ok.
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	b2, err := backblaze.NewB2(backblaze.Credentials{
		AccountID:      accountID,
		ApplicationKey: appKey,
	})
	if err != nil {
		return fmt.Errorf("Couldn't authorize: %v", err)
	}

	bucket, err = b2.Bucket(bucketName)
	if err != nil {
		return fmt.Errorf("Couldn't open bucket %#v: %v", bucketName, err)
	}

	if bucket == nil {
		fmt.Fprintf(os.Stderr, "Creating private B2 bucket %#v\n", bucketName)
		bucket, err = b2.CreateBucket(bucketName, backblaze.AllPrivate)
		if err != nil {
			return fmt.Errorf("Couldn't create bucket %#v: %v", bucketName, err)
		}
	}

	return nil
}

func store(key, file string) {
	ok := false
	defer func() {
		if ok {
			out.WriteString("TRANSFER-SUCCESS STORE ")
		} else {
			out.WriteString("TRANSFER-FAILURE STORE ")
		}
		out.WriteString(key)
		out.WriteString("\n")
	}()

	fh, err := os.Open(file)
	if err != nil {
		log.Printf("Couldn't open %v for reading: %v", file, err)
		return
	}
	defer fh.Close()

	shaReady := make(chan struct{})
	var haveSHA []byte
	var contentLength int64
	var shaError error
	go func() {
		defer close(shaReady)

		sha := sha1.New()
		n, err := io.Copy(sha, fh)
		if err != nil {
			shaError = err
			return
		}
		contentLength = n

		_, err = fh.Seek(0, 0)
		if err != nil {
			shaError = err
			return
		}

		haveSHA = sha.Sum(nil)
	}()

	res, err := bucket.ListFileNames(prefix+key, 1)
	if err != nil {
		log.Printf("Couldn't list filenames: %v", err)
		return
	}

	if len(res.Files) > 0 && res.Files[0].Name == prefix+key {
		// file probably already stored; make sure using the SHA1
		b2file, err := bucket.GetFileInfo(res.Files[0].ID)
		if err != nil {
			log.Printf("Couldn't get file info for %v: %v", res.Files[0].ID, err)
			return
		}
		if b2file != nil {
			<-shaReady

			wantSHA, err := hex.DecodeString(b2file.ContentSha1)
			if err == nil && bytes.Equal(haveSHA, wantSHA) {
				ok = true
				return
			}

			// File exists but is the incorrect data. Delete the old version
			// first.
			_, err = bucket.DeleteFileVersion(prefix+key, b2file.ID)
			if err != nil {
				log.Printf("Couldn't delete old file version: %v", err)
				return
			}
		}
	}

	<-shaReady
	if shaError != nil {
		log.Printf("Couldn't hash local file %v: %v", file, shaError)
		return
	}

	_, err = bucket.UploadHashedFile(prefix+key, nil, NewProgressReader(fh, out), hex.EncodeToString(haveSHA), contentLength)
	if err != nil {
		log.Printf("Couldn't upload file: %v", err)
		return
	}

	ok = true
}

func retrieve(key, file string) {
	ok := false
	defer func() {
		if ok {
			out.WriteString("TRANSFER-SUCCESS RETRIEVE ")
		} else {
			out.WriteString("TRANSFER-FAILURE RETRIEVE ")
		}
		out.WriteString(key)
		out.WriteString("\n")
	}()

	_, rc, err := bucket.DownloadFileByName(prefix + key)
	if rc != nil {
		defer rc.Close()
	}
	if err != nil {
		log.Printf("Couldn't download file: %v", err)
		return
	}

	fh, err := os.Create(file)
	if err != nil {
		log.Printf("Couldn't open %v for writing: %v", file, err)
		return
	}
	defer fh.Close()

	_, err = io.Copy(fh, NewProgressReader(rc, out))
	if err != nil {
		log.Printf("Couldn't download file: %v", err)
		return
	}

	ok = true
}

func checkPresent(key string) {
	ret := "UNKNOWN"
	defer func() {
		out.WriteString("CHECKPRESENT-" + ret + " " + key + "\n")
	}()

	res, err := bucket.ListFileNames(prefix+key, 1)
	if err != nil {
		log.Printf("Couldn't list filenames: %v", err)
		return
	}

	if len(res.Files) == 0 || res.Files[0].Name != prefix+key {
		ret = "FAILURE"
	} else {
		ret = "SUCCESS"
	}
}

func remove(key string) {
	ret := "FAILURE"
	defer func() {
		out.WriteString("REMOVE-" + ret + " " + key + "\n")
	}()

	res, err := bucket.ListFileNames(prefix+key, 1)
	if err != nil {
		log.Printf("Couldn't list filenames: %v", err)
		return
	}

	if len(res.Files) == 0 || res.Files[0].Name != prefix+key {
		// File already non-existent
		ret = "SUCCESS"
		return
	}

	_, err = bucket.DeleteFileVersion(res.Files[0].Name, res.Files[0].ID)
	if err != nil {
		log.Printf("Couldn't delete file version: %v", err)
		return
	}

	ret = "SUCCESS"
}

func main() {
	out.Write([]byte("VERSION 1\n"))

	for {
		out.Flush()

		line, err := in.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}

			log.Fatal(err)
		}
		line = strings.TrimSuffix(line, "\n")

		switch {
		case strings.HasPrefix(line, "INITREMOTE"):
			prepare("INITREMOTE")

		case strings.HasPrefix(line, "PREPARE"):
			prepare("PREPARE")

		case strings.HasPrefix(line, "TRANSFER "):
			line = strings.TrimPrefix(line, "TRANSFER ")

			fields := strings.SplitN(line, " ", 3)
			for len(fields) < 3 {
				fields = append(fields, "")
			}

			switch fields[0] {
			case "STORE":
				store(fields[1], fields[2])
			case "RETRIEVE":
				retrieve(fields[1], fields[2])
			default:
				out.WriteString("UNSUPPORTED-REQUEST\n")
			}

		case strings.HasPrefix(line, "CHECKPRESENT "):
			key := strings.TrimPrefix(line, "CHECKPRESENT ")
			checkPresent(key)

		case strings.HasPrefix(line, "REMOVE "):
			key := strings.TrimPrefix(line, "REMOVE ")
			remove(key)

		default:
			out.WriteString("UNSUPPORTED-REQUEST\n")
		}
	}
}
