# Humble Bundle File Downloader


Forked from: https://github.com/diogogmt/humblebundle-downloader for my own need.

I download all the ebooks I own and being able to:
- filter ebook (achieved with -platfom flag)
- download a single format for comic bundles  (achieved with -only and -onlyif flags )
- filter the mobi format I don't use (achieved iwth -exclude flag)


## Usage

the auth http only cookie is needed for authentification and can be extracted from developer console

note: a subfolder is created for each bundle


```shell
Usage: hb-downloader
  -all
        download all bundles
  -auth string
        Account _simpleauth_sess cookie
  -exclude string
        exclude a format from the downloads. ex: mobi
  -ifonly
        'only' flag will be used on the condition the precised format is available for a given download
  -key string
        key: Key listed in the URL params in the downloads page
  -list
        list the bundles
  -only string
        only download a certain format. ex: cbz
  -out string
        out: /path/to/save/books
  -platform string
        filter by platform ex: ebook
```

### Usage exemples

#### list all your bundles
 ```shell
 hb-downloader -auth "..." -list
 ```

#### download all ebooks from every bundle excluding mobi format and only downloading cbz format if present
 ```shell
 hb-downloader -auth "..." -out "~/humblebundle" --platform ebook -exclude mobi -only cbz -ifonly all
 ```

#### download all eboks from a specific bundle
```shell
 hb-downloader -auth "..." -out "~/humblebundle" -platform ebook -key "..."
```
