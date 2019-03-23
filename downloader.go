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

var (
	errInvalidContentDispositionHeader  = fmt.Errorf("Invalid Content-Dispostion Header")
	errContentDispositionHeaderNotFound = fmt.Errorf("Content-Dispostion Header not found")
)

type FileDownloader struct {
	DownloadType humbleDownloadType
	dirPath      string
	altName      string
	httpName     string
}

type BundleDownloader struct {
	hbAPI     *HumbleBundleAPI
	parentDir string
	only      string
	ifOnly    bool
	exclude   string
}

func NewBundleDownloader(hbAPI *HumbleBundleAPI, parentDir string) *BundleDownloader {
	return &BundleDownloader{
		hbAPI:     hbAPI,
		parentDir: parentDir,
		ifOnly:    false,
	}
}

func (bd *BundleDownloader) SetOnlyFormatFilter(only string, ifOnly bool) {
	bd.only = only
	bd.ifOnly = ifOnly
}

func (bd *BundleDownloader) SetExcludeFormatFilter(exclude string) {
	bd.exclude = exclude
}

func (bd *BundleDownloader) Download(order humbleBundleOrder) error {

	name := order.Product.HumanName
	if name == "" {
		name = order.Product.MachineName
	}

	outputDir := path.Join(bd.parentDir, removeIllegalCharacters(name))

	// 1 - Iterate through all products
	tasks := []*Task{}
	for _, product := range order.Products {
		for _, download := range product.Downloads {
			if *platform != "" && *platform != download.Platform {
				continue
			}
			// 3 - Iterate through download types (PDF,EPUB,MOBI)
			fileDownloads := make([]*FileDownloader, 0, len(download.DownloadTypes))
			onlyPresent := false
			for _, DownloadType := range download.DownloadTypes {
				if bd.exclude != "" && DownloadType.fileExtension() == bd.exclude {
					continue
				}
				// ifOnly not present, filter right now
				if !bd.ifOnly && bd.only != "" && DownloadType.fileExtension() != bd.only {
					continue

				}
				// ifOnly present memorize if the format is present
				if bd.ifOnly && bd.only != "" && DownloadType.fileExtension() == bd.only {
					onlyPresent = true
				}
				fd := NewFileDownloader(DownloadType, outputDir, product.HumanName)
				fileDownloads = append(fileDownloads, fd)

			}

			// ifOnly is set, the format is present: filter the results
			if bd.ifOnly && onlyPresent && bd.only != "" {
				filteredDownloads := make([]*FileDownloader, 0, len(fileDownloads))
				for _, fileDownload := range fileDownloads {
					if fileDownload.DownloadType.fileExtension() == bd.only {
						filteredDownloads = append(filteredDownloads, fileDownload)
					}
				}
				fileDownloads = filteredDownloads
			}

			for _, downloader := range fileDownloads {
				tasks = append(tasks, NewTask(func() error { return downloader.Download() }))
			}

		}
	}
	if len(tasks) == 0 {
		return nil
	}

	fmt.Printf("start downloading order '%s'\n", name)
	p := NewPool(tasks, 4)
	p.Run()
	var numErrors int
	for _, task := range p.Tasks {
		if task.Err != nil {
			log.Print(task.Err)
			numErrors++
		}
		if numErrors >= 10 {
			log.Print("Too many errors.")
			break
		}
	}
	if numErrors > 0 {
		return fmt.Errorf("order download failed")
	}
	return nil
}

func NewFileDownloader(DownloadType humbleDownloadType, dirPath string, altName string) *FileDownloader {
	return &FileDownloader{
		DownloadType: DownloadType,
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
	return fmt.Sprintf("%s.%s", removeIllegalCharacters(fd.name()), fd.DownloadType.fileExtension())
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
	if fd.DownloadType.SHA1 != "" {
		sha1String := fmt.Sprintf("%x", sha1sum.Sum(nil))
		if fd.DownloadType.SHA1 == sha1String {
			return true, nil
		}
		// fallback to MD5 due to sha1 errors
		md5String := fmt.Sprintf("%x", md5sum.Sum(nil))
		return fd.DownloadType.MD5 == md5String, nil
	}
	md5String := fmt.Sprintf("%x", md5sum.Sum(nil))
	return fd.DownloadType.MD5 == md5String, nil

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
	resp, err := http.Get(fd.DownloadType.URL.Web)
	if err != nil {
		return fmt.Errorf("error downloading file %s", fd.DownloadType.URL.Web)
	}
	defer resp.Body.Close()

	fd.getHTTPName(resp.Header)

	// if the file exist and the checksum is good don't download the file
	fi, err := os.Stat(fd.filepath())
	if err == nil && fi.Size() == fd.DownloadType.FileSize {
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
