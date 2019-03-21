package main

import (
	"crypto/md5"
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
)

func isValidChecksum(downloadType humbleDownloadType, filepath string) (bool, error) {
	f, err := os.Open(filepath)
	if err != nil {
		log.Printf("error reading file: %v for: %s", err, filepath)
	}
	defer f.Close()
	md5sum := md5.New()
	sha1sum := sha1.New()
	multiWriter := io.MultiWriter(md5sum, sha1sum)

	if _, err := io.Copy(multiWriter, f); err != nil {
		return false, fmt.Errorf("error calculating checksums: %v for: %s", err, filepath)
	}
	if downloadType.SHA1 != "" {
		sha1String := fmt.Sprintf("%x", sha1sum.Sum(nil))
		if downloadType.SHA1 == sha1String {
			return true, nil
		}
		// fallback to MD5 due to sha1 errors
		md5String := fmt.Sprintf("%x", md5sum.Sum(nil))
		return downloadType.MD5 == md5String, nil
	}
	md5String := fmt.Sprintf("%x", md5sum.Sum(nil))
	return downloadType.MD5 == md5String, nil

}

func syncFile(outputDir, filename, downloadURL string, downloadType humbleDownloadType) error {
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("error downloading file %s", downloadURL)
	}
	defer resp.Body.Close()

	// if content disposition exists used it to name the file
	contentDisposition := resp.Header.Get("Content-Disposition")
	if contentDisposition != "" {
		re := regexp.MustCompile(`filename="(.*)"`)
		matches := re.FindStringSubmatch(contentDisposition)
		if len(matches) == 1 {
			filename = matches[0]
		}
	}

	filepath := path.Join(outputDir, filename)

	// if the file exist and the checksum is good don't download the file
	_, err = os.Stat(filepath)
	if err == nil {
		ok, err := isValidChecksum(downloadType, filepath)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}

	bookLastmod := resp.Header.Get("Last-Modified")
	bookLastmodTime, err := http.ParseTime(bookLastmod)
	if err != nil {
		return fmt.Errorf("error reading Last-Modified header data: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("error status code %d", resp.StatusCode)
	}

	// create parent directory if needed
	err = os.MkdirAll(outputDir, 0777)
	if err != nil {
		return fmt.Errorf("error creating order directory '%s'", outputDir)
	}

	bookFile, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("error creating book file (%s): %v", filepath, err)
	}
	defer bookFile.Close()
	_, err = io.Copy(bookFile, resp.Body)
	if err != nil {
		return fmt.Errorf("error copying response body to file '%s': '%v'", filepath, err)
	}
	os.Chtimes(filepath, bookLastmodTime, bookLastmodTime)

	ok, err := isValidChecksum(downloadType, filepath)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invalid checksum for file: %s", filepath)
	}
	return nil
}
