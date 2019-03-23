package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"gopkg.in/cheggaaa/pb.v1"
)

var (
	gameKey       = flag.String("key", "", "key: Key listed in the URL params in the downloads page")
	sessCookie    = flag.String("auth", "", "Account _simpleauth_sess cookie")
	out           = flag.String("out", "", "out: /path/to/save/books")
	all           = flag.Bool("all", false, "download all bundles")
	platform      = flag.String("platform", "", "filter by platform ex: ebook")
	excludeFormat = flag.String("exclude", "", "exclude a format from the downloads. ex: mobi")
	onlyFormat    = flag.String("only", "", "only download a certain format. ex: cbz")
	ifOnly        = flag.Bool("ifonly", false, "'only' flag will be used on the condition the precised format is available for a given download")
)

func downloadAll(hbAPI *HumbleBundleAPI, bundleDownloader *BundleDownloader) error {
	fmt.Println("downloading orders details...")

	keys, err := hbAPI.GetOrders()
	if err != nil {
		return err
	}
	orders := make([]humbleBundleOrder, 0, len(keys))
	progressbar := pb.New(len(keys))
	progressbar.Start()
	for _, key := range keys {
		order, err := hbAPI.GetOrder(key)
		if err != nil {
			return err
		}
		orders = append(orders, order)
		progressbar.Increment()

	}
	progressbar.Finish()

	var lastError error
	for _, order := range orders {
		err := bundleDownloader.Download(order)
		if err != nil {
			lastError = err
		}
	}
	return lastError
}

func downloadOrder(hbAPI *HumbleBundleAPI, bundleDownloader *BundleDownloader, key string) error {

	order, err := hbAPI.GetOrder(key)
	if err != nil {
		return err
	}
	bundleDownloader.Download(order)
	return nil
}

func main() {
	flag.Parse()

	if *sessCookie == "" {
		log.Println("Missing _simpleauth_sess auth cookie")
		flag.Usage()
		os.Exit(-1)
	}

	hbAPI := NewHumbleBundleAPI(*sessCookie)

	// cleanup only/exclude formats
	only := strings.ToLower(*onlyFormat)
	exclude := strings.ToLower(*excludeFormat)

	bundleDownloader := NewBundleDownloader(hbAPI, *out)
	bundleDownloader.SetOnlyFormatFilter(only, *ifOnly)
	bundleDownloader.SetExcludeFormatFilter(exclude)

	var err error

	if *all == true {
		err = downloadAll(hbAPI, bundleDownloader)
	} else {
		if *gameKey == "" {
			log.Println("Missing key")
			flag.Usage()
		}
		err = downloadOrder(hbAPI, bundleDownloader, *gameKey)
	}

	if err != nil {
		log.Println(err)
		os.Exit(-1)
	}
}
