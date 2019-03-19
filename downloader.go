package main

import (
	"crypto/md5"
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
)

func isValidChecksum(downloadType humbleDownloadType, filepath string) bool {
	f, err := os.Open(filepath)
	if err != nil {
		log.Printf("error reading file: %v for: %s", err, filepath)
	}
	defer f.Close()
	var h hash.Hash
	if downloadType.SHA1 != "" {
		h = sha1.New()
	} else {
		h = md5.New()
	}

	if _, err := io.Copy(h, f); err != nil {
		log.Printf("error calculating sha1sum: %v for: %s", err, filepath)
	}
	bs := h.Sum(nil)
	bsString := fmt.Sprintf("%x", bs)
	if downloadType.SHA1 != "" {
		return downloadType.SHA1 == bsString
	}
	return downloadType.MD5 == bsString

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
	if err == nil && isValidChecksum(downloadType, filepath) {
		log.Printf("skipping already downloaded file: '%s'\n", filename)
		return nil
	}

	bookLastmod := resp.Header.Get("Last-Modified")
	bookLastmodTime, err := http.ParseTime(bookLastmod)
	if err != nil {
		return fmt.Errorf("error reading Last-Modified header data: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("error status code %d", resp.StatusCode)
	}

	bookFile, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("error creating book file (%s): %v", filepath, err)
	}
	defer bookFile.Close()
	log.Printf("starting download: '%s'\n", filename)
	_, err = io.Copy(bookFile, resp.Body)
	if err != nil {
		return fmt.Errorf("error copying response body to file '%s': '%v'", filepath, err)
	}
	log.Printf("Finished saving file %s", filepath)
	os.Chtimes(filepath, bookLastmodTime, bookLastmodTime)

	if !isValidChecksum(downloadType, filepath) {
		return fmt.Errorf("invalid checksum for file: %s", filepath)
	}

	return nil
}
