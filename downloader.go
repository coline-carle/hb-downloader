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
	"strings"
)

var (
	errInvalidContentDispositionHeader  = fmt.Errorf("Invalid Content-Dispostion Header")
	errContentDispositionHeaderNotFound = fmt.Errorf("Content-Dispostion Header not found")
)

type FileDownloader struct {
	downloadType humbleDownloadType
	dirPath      string
	altName      string
	httpName     string
}

func NewFileDownloader(downloadType humbleDownloadType, dirPath string, altName string) *FileDownloader {
	return &FileDownloader{
		downloadType: downloadType,
		dirPath:      dirPath,
		altName:      altName,
	}
}

func (fd *FileDownloader) name() string {
	if fd.httpName != "" {
		return fd.httpName
	}
	return fd.altName
}

func (fd *FileDownloader) filename() string {
	return fmt.Sprintf("%s.%s", removeIllegalCharacters(fd.name()), fd.downloadType.fileExtension())
}

func (fd *FileDownloader) filepath() string {
	return path.Join(fd.dirPath, fd.filename())
}

func (fd *FileDownloader) isDownloadValid() (bool, error) {
	f, err := os.Open(fd.filepath())
	if err != nil {
		log.Printf("error reading file: %v for: %s", err, fd.filepath())
	}
	defer f.Close()
	md5sum := md5.New()
	sha1sum := sha1.New()
	multiWriter := io.MultiWriter(md5sum, sha1sum)

	if _, err := io.Copy(multiWriter, f); err != nil {
		return false, fmt.Errorf("error calculating checksums: %v for: %s", err, fd.filepath())
	}
	if fd.downloadType.SHA1 != "" {
		sha1String := fmt.Sprintf("%x", sha1sum.Sum(nil))
		if fd.downloadType.SHA1 == sha1String {
			return true, nil
		}
		// fallback to MD5 due to sha1 errors
		md5String := fmt.Sprintf("%x", md5sum.Sum(nil))
		return fd.downloadType.MD5 == md5String, nil
	}
	md5String := fmt.Sprintf("%x", md5sum.Sum(nil))
	return fd.downloadType.MD5 == md5String, nil

}

func (fd *FileDownloader) getHTTPName(header http.Header) (err error) {
	contentDisposition := header.Get("Content-Disposition")
	if contentDisposition != "" {
		re := regexp.MustCompile(`filename="(.+)"`)
		matches := re.FindStringSubmatch(contentDisposition)
		if len(matches) == 1 {
			fd.httpName = removeIllegalCharacters(matches[0])
			return nil
		}
		return errInvalidContentDispositionHeader
	}
	return errContentDispositionHeaderNotFound
}

func (fd *FileDownloader) fixLastModified(header http.Header) error {
	lastMod := header.Get("Last-Modified")
	lastModTime, err := http.ParseTime(lastMod)
	if err != nil {
		return fmt.Errorf("error reading Last-Modified header data: %v", err)
	}
	os.Chtimes(fd.filepath(), lastModTime, lastModTime)
	return nil
}

func (fd *FileDownloader) Download() error {
	resp, err := http.Get(fd.downloadType.URL.Web)
	if err != nil {
		return fmt.Errorf("error downloading file %s", fd.downloadType.URL.Web)
	}
	defer resp.Body.Close()

	fd.getHTTPName(resp.Header)

	// if the file exist and the checksum is good don't download the file
	fi, err := os.Stat(fd.filepath())
	if err == nil && fi.Size() == fd.downloadType.FileSize {
		ok, _ := fd.isDownloadValid()
		if ok {
			return nil
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("error status code %d", resp.StatusCode)
	}

	// create parent directory if needed
	err = os.MkdirAll(fd.dirPath, 0777)
	if err != nil {
		return fmt.Errorf("error creating order directory '%s'", fd.dirPath)
	}

	bookFile, err := os.Create(fd.filepath())
	if err != nil {
		return fmt.Errorf("error creating book file (%s): %v", fd.filepath(), err)
	}
	defer bookFile.Close()
	_, err = io.Copy(bookFile, resp.Body)
	if err != nil {
		return fmt.Errorf("error copying response body to file '%s': '%v'", fd.filepath(), err)
	}

	ok, err := fd.isDownloadValid()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invalid checksum for file: %s", fd.filepath())
	}

	err = fd.fixLastModified(resp.Header)
	if err != nil {
		return err
	}

	return nil
}

func removeIllegalCharacters(filename string) string {
	filename = strings.Replace(filename, "/", "_", -1)
	filename = strings.Replace(filename, ":", " ", -1)
	filename = strings.Replace(filename, "!", " ", -1)
	filename = strings.Replace(filename, "?", " ", -1)
	return filename
}
