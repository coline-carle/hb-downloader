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
	gameKey    = flag.String("key", "", "key: Key listed in the URL params in the downloads page")
	sessCookie = flag.String("auth", "", "Account _simpleauth_sess cookie")
	out        = flag.String("out", "", "out: /path/to/save/books")
	all        = flag.Bool("all", false, "download all purshases")
	platform   = flag.String("platform", "", "filter by platform ex: ebook")
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

func downloadOrder(order humbleBundleOrder, parentDir string) {

	name := order.Product.HumanName
	if name == "" {
		name = order.Product.MachineName
	}

	outputDir := path.Join(parentDir, removeIllegalCharacters(name))

	// 1 - Iterate through all products
	tasks := []*Task{}
	for i := 0; i < len(order.Products); i++ {
		prod := order.Products[i]
		// 2 - Iterate through the product downloads, currently only returns ebook platform download

		for j := 0; j < len(prod.Downloads); j++ {
			download := prod.Downloads[j]
			// 3 - Iterate through download types (PDF,EPUB,MOBI)
			for x := 0; x < len(download.DownloadTypes); x++ {
				if *platform != "" && *platform != download.Platform {
					continue
				}
				downloadType := download.DownloadTypes[x]
				filename := fmt.Sprintf("%s.%s", prod.HumanName, strings.ToLower(strings.TrimPrefix(downloadType.Name, ".")))
				// cleanup filename
				filename = removeIllegalCharacters(filename)
				filename = strings.Replace(filename, ".supplement", "_supplement.zip", 1)
				filename = strings.Replace(filename, ".download", "_video.zip", 1)

				tasks = append(tasks, NewTask(func() error { return syncFile(outputDir, filename, downloadType.URL.Web, downloadType) }))
			}
		}
	}
	if len(tasks) == 0 {
		return
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

	if *all == true {
		keys := getOrderList(*sessCookie)
		for _, key := range keys {
			order, err := fetchOrder(key, *sessCookie)
			if err != nil {
				log.Println(err)
				return
			}
			downloadOrder(order, *out)
		}
		os.Exit(0)
	}
	if *gameKey == "" {
		log.Println("Missing key")
		flag.Usage()
	}
	order, err := fetchOrder(*gameKey, *sessCookie)
	if err != nil {
		log.Println(err)
		os.Exit(-1)
	}
	downloadOrder(order, *out)

}
