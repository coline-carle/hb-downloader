package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
)

const (
	baseURL        = "https://www.humblebundle.com/api/v1"
	orderPath      = "/order"
	userOrdersPath = "/user/order"
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

type HumbleBundleAPI struct {
	sessCookie string
}

func NewHumbleBundleAPI(sessCookie string) *HumbleBundleAPI {
	return &HumbleBundleAPI{
		sessCookie,
	}
}
func (hb *HumbleBundleAPI) AuthGet(apiPath string) (*http.Response, error) {
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
		Value: hb.sessCookie,
	}
	req.AddCookie(cookie)

	// Fetch order information
	client := &http.Client{}
	return client.Do(req)
}

func (hb *HumbleBundleAPI) GetOrders() (keys []string, err error) {
	resp, err := hb.AuthGet(userOrdersPath)
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
		return nil, fmt.Errorf("error unmarshaling orders list: %v", err)
	}
	keys = make([]string, 0, len(orders))
	for _, order := range orders {
		keys = append(keys, order.Gamekey)
	}
	return keys, nil
}

func (hb *HumbleBundleAPI) GetOrder(key string) (humbleBundleOrder, error) {
	order := humbleBundleOrder{}
	apiPath := path.Join(orderPath, key)
	resp, err := hb.AuthGet(apiPath)
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
