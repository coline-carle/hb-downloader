package main

import (
	"errors"
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
	list          = flag.Bool("list", false, "list the bundles")
)

func startDownload(title string, downloads []*FileDownloader) error {
	fmt.Printf("Downloading '%s'...\n", title)
	return download(downloads)
}

func download(downloads []*FileDownloader) error {

	if len(downloads) == 0 {
		return nil
	}

	tasks := make([]*Task, len(downloads), len(downloads))
	for i, downloader := range downloads {
		tasks[i] = NewTask(func() error { return downloader.Download() })
	}

	p := NewPool(tasks, 4)
	p.Run()

	// process errors
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
		return fmt.Errorf("at least %d download(s) have failed", numErrors)
	}

	return nil
}

func getBundlesDetails(hbAPI *HumbleBundleAPI) (order []humbleBundleOrder, err error) {
	fmt.Println("Downloading bundles details...")

	keys, err := hbAPI.GetOrders()
	if err != nil {
		return nil, err
	}
	var lastError error

	orders := make([]humbleBundleOrder, 0, len(keys))
	progressbar := pb.New(len(keys))
	progressbar.Start()
	for _, key := range keys {
		order, err := hbAPI.GetOrder(key)
		if err != nil {
			lastError = err
		}
		orders = append(orders, order)
		progressbar.Increment()

	}
	progressbar.Finish()
	return orders, lastError
}

func downloadAllBundles(hbAPI *HumbleBundleAPI, bundleDownloader *BundleDownloader) error {
	var lastError error
	bundles, err := getBundlesDetails(hbAPI)
	if err != nil {
		return err
	}

	for _, bundle := range bundles {
		downloads := bundleDownloader.Downloads(bundle)
		err = startDownload(bundle.Product.HumanName, downloads)
		if err != nil {
			lastError = err
		}
	}
	if lastError != nil {
		return errors.New("at least one have download failed")
	}
	return nil
}

func printBundleTitle(bundle humbleBundleOrder) {
	fmt.Printf("%s [%s]\n", bundle.Product.HumanName, bundle.GameKey)
}

func listBundles(hbAPI *HumbleBundleAPI) error {
	bundles, err := getBundlesDetails(hbAPI)
	if err != nil {
		return err
	}
	for _, bundle := range bundles {
		printBundleTitle(bundle)
	}
	return nil
}

func downloadBundle(hbAPI *HumbleBundleAPI, bundleDownloader *BundleDownloader, key string) error {
	bundle, err := hbAPI.GetOrder(key)
	if err != nil {
		return err
	}
	downloads := bundleDownloader.Downloads(bundle)
	return startDownload(bundle.Product.HumanName, downloads)
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

	switch {
	case *list:
		err = listBundles(hbAPI)
	case *all:
		err = downloadAllBundles(hbAPI, bundleDownloader)
	default:
		if *gameKey == "" {
			log.Println("Missing key")
			flag.Usage()
		}
		err = downloadBundle(hbAPI, bundleDownloader, *gameKey)
	}

	if err != nil {
		log.Println(err)
		os.Exit(-1)
	}
}
