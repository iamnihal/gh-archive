package main

import (
	"encoding/json"
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

func parseCMDLineArgs() (string, int, int, string, string) {
	var t string
	var n int
	var o string
	var l string

	flag.StringVar(&t, "t", "", "Repository topic (for eg: javascript, python, ai, ml, etc)")
	flag.IntVar(&n, "n", 0, "Number of repositories to check")
	flag.StringVar(&o, "o", "", "Save archived repository result to a file")
	flag.StringVar(&l, "l", "", "Save all repositories list to a file")

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
	return t, pageNo, n, o, l
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

func (s *Scraper) extractRepoURL(data string, m *[]string) {
	matches := s.repoURL.FindAllStringSubmatch(data, -1)

	for _, match := range matches {
		if len(match) > 1 {
			*m = append(*m, match[1])
		}
	}

}

func (s *Scraper) extractStarCount(data string) string {
	return s.repoStar.FindStringSubmatch(data)[1]
}

func (s *Scraper) isRepoArchived(data []string) map[string]string {
	var archivedRepo = make(map[string]string)
	for i, repo := range data {
		fmt.Printf("\r[INFO] Checking for archived repository [%d/%d]", i, len(data))
		u := "https://github.com" + repo
		res := s.httpGetRequest(u)
		matched := s.isArchiveRegex.MatchString(res)
		if matched {
			star := s.extractStarCount(res)
			archivedRepo[u] = star
			fmt.Printf("\n[+] %s [ARCHIVED] : %s\n", u, star)
		}
	}
	fmt.Println("\n[INFO] Operation Completed")
	return archivedRepo
}

func (s *Scraper) saveOutput(d map[string]string, f string) {
	data, err := json.Marshal(d)

	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(f, data, 0644)

	if err != nil {
		log.Fatal(err)
	}
}

func (s *Scraper) saveRepoList(d []string, f string) {
	data, err := json.MarshalIndent(d, "", "    ")

	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(f, data, 0644)

	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	t, pageNo, n, o, l := parseCMDLineArgs()
	scraper := NewScraper()
	m := make([]string, 0, n)

	fmt.Printf("[INFO] Fetching list of top %d repositories from %s topic\n", n, t)

	for i := 1; i <= pageNo; i++ {
		url := fmt.Sprintf("https://github.com/topics/%s?l=%s&page=%d", t, t, i)
		res := scraper.httpGetRequest(url)
		scraper.extractRepoURL(res, &m)
	}

	m = m[:n]

	fmt.Printf("[INFO] Repositories list extracted successfully\n")

	a := scraper.isRepoArchived(m)

	if o != "" {
		scraper.saveOutput(a, o)
	}

	if l != "" {
		scraper.saveRepoList(m, l)
	}

	if len(a) == 0 {
		fmt.Println("[-] No archived repository found")
	}
}
