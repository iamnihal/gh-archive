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

type Scraper struct {
	client         *http.Client
	repoURL        *regexp.Regexp
	repoStar       *regexp.Regexp
	isArchiveRegex *regexp.Regexp
	isArchive      bool
}

func NewScraper() *Scraper {
	return &Scraper{
		client:         &http.Client{},
		repoURL:        regexp.MustCompile(`<a\sid="code-tab-.*"[^>]*href="(.[^"]*)`),
		repoStar:       regexp.MustCompile(`<span\sid="repo-stars-counter-star"[^>]*>(.*)<\/span>`),
		isArchiveRegex: regexp.MustCompile(`It is now read-only`),
		isArchive:      false,
	}
}

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

func (s *Scraper) httpGetRequest(u string) string {
	res, err := s.client.Get(u)
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

func (s *Scraper) extractRepoURL(data string) []string {
	matches := s.repoURL.FindAllStringSubmatch(data, -1)
	var extractedMatches []string

	for _, match := range matches {
		if len(match) > 1 {
			extractedMatches = append(extractedMatches, match[1])
		}
	}

	return extractedMatches
}

func (s *Scraper) extractStarCount(data string) string {
	return s.repoStar.FindStringSubmatch(data)[1]
}

func (s *Scraper) isRepoArchived(data []string) map[string]string {
	var archivedRepo = make(map[string]string)
	for _, repo := range data {
		u := "https://github.com" + repo
		fmt.Print("Checking [", u, "]\n")
		res := s.httpGetRequest(u)
		matched := s.isArchiveRegex.MatchString(res)
		if matched {
			star := s.extractStarCount(res)
			archivedRepo[u] = star
		}
	}
	return archivedRepo
}

func main() {
	t, pageNo := parseCMDLineArgs()
	scraper := NewScraper()
	fmt.Println("# Searching for ARCHIVED Repositories")

	for i := 1; i <= pageNo; i++ {
		url := fmt.Sprintf("https://github.com/topics/%s?l=%s&page=%d", t, t, i)
		res := scraper.httpGetRequest(url)
		u := scraper.extractRepoURL(res)
		a := scraper.isRepoArchived(u)
		if len(a) == 0 {
			fmt.Println("[-] No archived repository found")
		} else {
			for repo, star := range a {
				fmt.Printf("[+] %s [ARCHIVED] : %s\n", repo, star)
			}
		}
	}
}
