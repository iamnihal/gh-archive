package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
)

func parseCMDLineArgs() (string, int) {
	var t string
	var n int

	flag.StringVar(&t, "t", "", "Repository topic (for eg: javascript, python, ai, ml, etc)")
	flag.IntVar(&n, "n", 0, "Number of repositories to check")

	flag.Usage = func() {
		fmt.Println("Options:")
		flag.PrintDefaults()
	}

	flag.Parse()

	if t == "" {
		println("Topic argument is compulsory")
		flag.Usage()
		os.Exit(1)
	}

	var pageNo int
	if n > 20 {
		pageNo = n / 20
		if n%20 != 0 {
			pageNo++
		}
	} else {
		pageNo = 1
	}
	return t, pageNo
}

func httpGetRequest(u string) string {
	res, err := http.Get(u)
	if err != nil {
		log.Fatal(err)
	}
	data, err := io.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	return string(data)
}

func extractRepoURL(s string, re *regexp.Regexp) []string {
	matches := re.FindAllStringSubmatch(s, -1)
	var extractedMatches []string

	for _, match := range matches {
		if len(match) > 1 {
			extractedMatches = append(extractedMatches, match[1])
		}
	}

	return extractedMatches
}

func extractStarCount(s string) string {
	re, err := regexp.Compile(`<span\sid="repo-stars-counter-star"[^>]*>(.*)<\/span>`)
	if err != nil {
		log.Fatal(err)
	}
	return re.FindStringSubmatch(s)[1]
}

func isRepoArchived(s []string) map[string]string {
	re, err := regexp.Compile(`It is now read-only`)
	if err != nil {
		log.Fatal(err)
	}
	var archivedRepo = make(map[string]string)
	for _, repo := range s {
		u := "https://github.com" + repo
		fmt.Print("Checking [", u, "]\n")
		data := httpGetRequest(u)
		matched := re.MatchString(data)
		if matched {
			star := extractStarCount(data)
			archivedRepo[u] = star
		}
	}
	return archivedRepo
}

func main() {

	t, pageNo := parseCMDLineArgs()

	regexPattern := `<a\sid="code-tab-.*"[^>]*href="(.[^"]*)`
	re := regexp.MustCompile(regexPattern)

	fmt.Println("# Searching for ARCHIVED Repositories")

	for i := 1; i <= pageNo; i++ {
		url := fmt.Sprintf("https://github.com/topics/%s?l=%s&page=%d", t, t, i)
		data := httpGetRequest(url)
		repoURLs := extractRepoURL(data, re)
		res := isRepoArchived(repoURLs)
		if len(res) == 0 {
			fmt.Println("[-] No archived repository found")
		} else {
			for repo, star := range res {
				fmt.Printf("[+] %s [ARCHIVED] : %s\n", repo, star)
			}
		}
	}
}
