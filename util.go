package main

import "strings"

func removeIllegalCharacters(filename string) string {
	filename = strings.Replace(filename, "/", "_", -1)
	filename = strings.Replace(filename, ":", " ", -1)
	filename = strings.Replace(filename, "!", " ", -1)
	filename = strings.Replace(filename, "?", " ", -1)
	return filename
}
