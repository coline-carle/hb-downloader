package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

const (
	baseURL        = "https://www.humblebundle.com/api/v1"
	orderPath      = "/order"
	userOrdersPath = "/user/order"
)

var (
	gameKey       = flag.String("key", "", "key: Key listed in the URL params in the downloads page")
	sessCookie    = flag.String("auth", "", "Account _simpleauth_sess cookie")
	out           = flag.String("out", "", "out: /path/to/save/books")
	all           = flag.Bool("all", false, "download all purshases")
	platform      = flag.String("platform", "", "filter by platform ex: ebook")
	excludeFormat = flag.String("exclude", "", "exclude a format from the downloads. ex: mobi")
	onlyFormat    = flag.String("only", "", "only download a certain format. ex: cbz")
)

type humbleBundleOrderKey struct {
	Gamekey string `json:"gamekey"`
}

type humbleDownloadType struct {
	SHA1 string `json:"sha1"`
	Name string `json:"name"`
	URL  struct {
		Web        string `json:"web"`
		BitTorrent string `json:"bittorrent"`
	} `json:"url"`
	HumanSize string `json:"human_size"`
	FileSize  int64  `json:"file_size"`
	MD5       string `json:"md5"`
}

func (hdt *humbleDownloadType) fileExtension() string {
	return strings.ToLower(strings.TrimPrefix(hdt.Name, "."))
}

type humbleBundleOrder struct {
	AmountSpent float64
	Product     struct {
		Category    string
		MachineName string
		HumanName   string `json:"human_name"`
	}
	GameKey  string `json:"gamekey"`
	UID      string `json:"uid"`
	Created  string `json:"created"`
	Products []struct {
		MachineName string `json:"machine_name"`
		HumanName   string `json:"human_name"`
		URL         string `json:"url"`
		Downloads   []struct {
			MachineName   string               `json:"machine_name"`
			HumanName     string               `json:"human_name"`
			Platform      string               `json:"platform"`
			DownloadTypes []humbleDownloadType `json:"download_struct"`
		} `json:"downloads"`
	} `json:"subproducts"`
}

type logger struct {
}

func (writer logger) Write(bytes []byte) (int, error) {
	return fmt.Print(string(bytes))
}

func authGet(apiPath string, session string) (*http.Response, error) {
	// Build endpoint URL
	u, err := url.Parse(baseURL)
	if err != nil {
		log.Fatal(err)
	}
	u.Path = path.Join(u.Path, apiPath)

	// prepare request
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		log.Fatalf("fatal error creating the request : %v", err)
	}
	// set session cookie
	cookie := &http.Cookie{
		Name:  "_simpleauth_sess",
		Value: *sessCookie,
	}
	req.AddCookie(cookie)

	// Fetch order information
	client := &http.Client{}
	return client.Do(req)
}

func getOrderList(session string) []string {
	resp, err := authGet(userOrdersPath, session)
	if err != nil {
		log.Fatalf("error downloading orders list: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Fatalf(" invalid http status for orders list: %s", resp.Status)
	}
	defer resp.Body.Close()
	buf, err := ioutil.ReadAll(resp.Body)
	orders := []humbleBundleOrderKey{}
	err = json.Unmarshal(buf, &orders)
	if err != nil {
		log.Fatalf("error unmarshaling orders list: %v", err)
	}
	keys := []string{}
	for _, order := range orders {
		keys = append(keys, order.Gamekey)
	}
	return keys
}

func fetchOrder(key string, session string) (humbleBundleOrder, error) {
	order := humbleBundleOrder{}
	apiPath := path.Join(orderPath, key)
	resp, err := authGet(apiPath, session)
	if err != nil {
		return order, fmt.Errorf("error downloading order information: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return order, fmt.Errorf("got invalid http status downloading order information : %s", resp.Status)
	}

	defer resp.Body.Close()
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return order, err
	}
	err = json.Unmarshal(buf, &order)
	return order, err
}

func downloadOrder(order humbleBundleOrder, parentDir, only, exclude string) error {

	name := order.Product.HumanName
	if name == "" {
		name = order.Product.MachineName
	}

	outputDir := path.Join(parentDir, removeIllegalCharacters(name))

	// 1 - Iterate through all products
	tasks := []*Task{}
	for _, product := range order.Products {
		for _, download := range product.Downloads {
			if *platform != "" && *platform != download.Platform {
				continue
			}
			// 3 - Iterate through download types (PDF,EPUB,MOBI)
			for _, downloadType := range download.DownloadTypes {
				if exclude != "" && downloadType.fileExtension() == exclude {
					continue
				}
				if only != "" && downloadType.fileExtension() != only {
					continue
				}
				fd := NewFileDownloader(downloadType, outputDir, product.HumanName)
				tasks = append(tasks, NewTask(func() error { return fd.Download() }))
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

func downloadAll(cookie, outputDir, only, exclude string) error {
	fmt.Println("downloading the list of orders...")
	keys := getOrderList(*sessCookie)
	orders := make([]humbleBundleOrder, 0, len(keys))
	for _, key := range keys {
		order, err := fetchOrder(key, cookie)
		if err != nil {
			return err
		}
		fmt.Printf("downloaded file list for order '%s'\n", order.Product.HumanName)
		orders = append(orders, order)

	}
	var lastError error
	for _, order := range orders {
		err := downloadOrder(order, outputDir, only, exclude)
		if err != nil {
			lastError = err
		}
	}
	return lastError
}

func downloadOrderWithKey(key, cookie, outputDir, only, exclude string) error {
	order, err := fetchOrder(key, cookie)
	if err != nil {
		return err
	}
	downloadOrder(order, outputDir, only, exclude)
	return nil
}

func main() {
	flag.Parse()

	log.SetFlags(0)
	log.SetOutput(new(logger))

	if *sessCookie == "" {
		log.Println("Missing _simpleauth_sess auth cookie")
		flag.Usage()
		os.Exit(-1)
	}

	var err error

	// cleanup only/exclude formats
	only := strings.ToLower(*onlyFormat)
	exclude := strings.ToLower(*excludeFormat)

	if *all == true {
		err = downloadAll(*sessCookie, *out, only, exclude)
	} else {
		if *gameKey == "" {
			log.Println("Missing key")
			flag.Usage()
		}
		err = downloadOrderWithKey(*gameKey, *sessCookie, *out, only, exclude)
	}

	if err != nil {
		log.Println(err)
		os.Exit(-1)
	}
}
