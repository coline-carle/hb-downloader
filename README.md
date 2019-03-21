# Humble Bundle File Downloader


Forked from: https://github.com/diogogmt/humblebundle-downloader for my own need: download all the ebook I own on humblebundle


## Usage

the auth http only cookie is needed for authentification

```shell
Usage: hb-downloader
  -all
        download all purshases
  -auth string
        Account _simpleauth_sess cookie
  -key string
        key: Key listed in the URL params in the downloads page
  -out string
        out: /path/to/save/books
  -platform string
        filter by platform ex: ebook
```

### download all bundles
 ```shell
 hb-downloader -auth "..." -out "~/humblebundle" -all -platform ebook
 ```

### download a specific bundle
```shell
 hb-downloader -auth "..." -out "~/humblebundle" -platform ebook -key "..."
```
