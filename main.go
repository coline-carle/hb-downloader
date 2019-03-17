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
	flags      = flag.NewFlagSet("humblebundle", flag.ExitOnError)
	gameKey    = flags.String("key", "", "key: Key listed in the URL params in the downloads page")
	sessCookie = flags.String("auth", "", "Account _simpleauth_sess cookie")
	out        = flags.String("out", "", "out: /path/to/save/books")
	all        = flags.Bool("all", false, "download all purshases")
	filter     = flags.String("filter", "", "filter downloads by extension ex: mobi")
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

func removeIllegalCharacters(filename string) string {
	filename = strings.Replace(filename, "/", "_", -1)
	filename = strings.Replace(filename, ":", ";", -1)
	filename = strings.Replace(filename, "!", "l", -1)
	return filename
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

func downloadOrder(key string, session string, parentDir string) {
	apiPath := path.Join(orderPath, key)
	resp, err := authGet(apiPath, session)
	if err != nil {
		log.Fatalf("error downloading order information: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Fatalf(" invalid http status for order information : %s", resp.Status)
	}

	defer resp.Body.Close()
	buf, err := ioutil.ReadAll(resp.Body)
	order := &humbleBundleOrder{}
	err = json.Unmarshal(buf, order)
	if err != nil {
		log.Fatalf("error unmarshaling order: %v", err)
	}
	name := order.Product.HumanName
	if name == "" {
		name = order.Product.MachineName
	}
	outputDir := path.Join(parentDir, removeIllegalCharacters(name))

	err = os.MkdirAll(outputDir, 0777)
	if err != nil {
		log.Fatalf("error creating parent directory '%s' for the bundle '%s'", parentDir, name)
	}
	log.Printf("downloading order '%s' into '%s'", name, outputDir)

	// 1 - Iterate through all products
	tasks := []*Task{}
	for i := 0; i < len(order.Products); i++ {
		prod := order.Products[i]
		// 2 - Iterate through the product downloads, currently only returns ebook platform download

		for j := 0; j < len(prod.Downloads); j++ {
			download := prod.Downloads[j]
			// 3 - Iterate through download types (PDF,EPUB,MOBI)
			for x := 0; x < len(download.DownloadTypes); x++ {
				downloadType := download.DownloadTypes[x]
				filename := fmt.Sprintf("%s.%s", prod.HumanName, strings.ToLower(strings.TrimPrefix(downloadType.Name, ".")))

				tasks = append(tasks, NewTask(func() error { return syncFile(outputDir, filename, downloadType.URL.Web, downloadType) }))
			}
		}
	}
	p := NewPool(tasks, 4)
	p.Run()
}

func main() {
	flags.Parse(os.Args[1:])

	log.SetFlags(0)
	log.SetOutput(new(logger))

	if *sessCookie == "" {
		log.Fatal("Missing _simpleauth_sess auth cookie")
	}

	if *all == true {
		keys := getOrderList(*sessCookie)
		for _, key := range keys {
			downloadOrder(key, *sessCookie, *out)
		}
		os.Exit(0)
	}
	if *gameKey == "" {
		log.Fatal("Missing key")
	}
	downloadOrder(*gameKey, *sessCookie, *out)

}
