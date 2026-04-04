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
	"sync"
	"time"

	"github.com/iamnihal/gh-archive/color"
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
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		repoURL:        regexp.MustCompile(`<a\sid="code-tab-.*"[^>]*href="(.[^"]*)`),
		repoStar:       regexp.MustCompile(`<span\sid="repo-stars-counter-star"[^>]*>(.*)<\/span>`),
		isArchiveRegex: regexp.MustCompile(`(?i)(read-only|archived|This repository has been archived)`),
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

func (s *Scraper) httpGetRequest(u string) (string, error) {
	fmt.Printf(color.Cyan+"[DEBUG] Fetching: %s\n"+color.Reset, u)

	res, err := s.client.Get(u)
	if err != nil {
		fmt.Printf(color.Red+"[ERROR] Failed to fetch %s: %v\n"+color.Reset, u, err)
		return "", err
	}
	defer res.Body.Close()

	fmt.Printf(color.Cyan+"[DEBUG] Response status: %d for %s\n"+color.Reset, res.StatusCode, u)

	data, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Printf(color.Red+"[ERROR] Failed to read body for %s: %v\n"+color.Reset, u, err)
		return "", err
	}
	return string(data), nil
}

func (s *Scraper) extractRepoURL(data string, m *[]string) {
	matches := s.repoURL.FindAllStringSubmatch(data, -1)
	fmt.Printf(color.Cyan+"[DEBUG] Found %d repo URLs in page\n"+color.Reset, len(matches))

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
	var mu sync.Mutex

	fmt.Printf(color.Yellow+"\n[INFO] Checking %d repositories for archive status\n"+color.Reset, len(data))

	var wg sync.WaitGroup
	for _, repo := range data {
		wg.Add(1)
		go func(repoURL string) {
			defer wg.Done()
			u := "https://github.com" + repoURL
			res, err := s.httpGetRequest(u)
			if err != nil {
				fmt.Printf(color.Red+"[ERROR] Failed to check %s: %v\n"+color.Reset, u, err)
				return
			}

			matched := s.isArchiveRegex.MatchString(res)
			fmt.Printf(color.Cyan+"[DEBUG] %s - Archived: %v\n"+color.Reset, u, matched)

			if matched {
				star := s.extractStarCount(res)
				if star == "" {
					star = "0"
				}
				fmt.Printf(color.Green+"[+] %s [ARCHIVED] : %s stars\n"+color.Reset, u, star)
				mu.Lock()
				archivedRepo[u] = star
				mu.Unlock()
			}
		}(repo)
	}

	wg.Wait()

	fmt.Printf(color.Yellow+"\n[INFO] Checked %d repos: %d archived\n"+color.Reset,
		len(data), len(archivedRepo))

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
	timeStart := time.Now()
	t, pageNo, n, o, l := parseCMDLineArgs()
	scraper := NewScraper()
	m := make([]string, 0, n)

	fmt.Printf(color.Yellow+"\n[INFO] Fetching top %d repositories from %s topic\n"+color.Reset, n, t)
	fmt.Printf(color.Cyan+"[DEBUG] Topic: %s, Pages: %d, Max repos: %d\n"+color.Reset, t, pageNo, n)

	for i := 1; i <= pageNo; i++ {
		url := fmt.Sprintf("https://github.com/topics/%s?l=%s&page=%d", t, t, i)
		fmt.Printf(color.Cyan+"[DEBUG] Fetching page %d/%d: %s\n"+color.Reset, i, pageNo, url)
		res, err := scraper.httpGetRequest(url)
		if err != nil {
			fmt.Printf(color.Red+"[ERROR] Failed to fetch page %d: %v\n"+color.Reset, i, err)
			continue
		}
		scraper.extractRepoURL(res, &m)
	}

	fmt.Printf(color.Cyan+"[DEBUG] Total repos found: %d\n"+color.Reset, len(m))

	if len(m) > n && n > 0 {
		m = m[:n]
	}

	fmt.Printf(color.Yellow+"[INFO] Repositories list extracted successfully (%d repos)\n"+color.Reset, len(m))

	if len(m) == 0 {
		fmt.Println(color.Red + "[-] No repositories found for this topic" + color.Reset)
		if o != "" {
			scraper.saveOutput(make(map[string]string), o)
		}
		return
	}

	a := scraper.isRepoArchived(m)

	fmt.Printf(color.Yellow+"\n[INFO] Found %d archived repositories\n"+color.Reset, len(a))

	if o != "" {
		fmt.Printf(color.Cyan+"[DEBUG] Saving output to: %s\n"+color.Reset, o)
		scraper.saveOutput(a, o)
	}
	if l != "" {
		fmt.Printf(color.Cyan+"[DEBUG] Saving repo list to: %s\n"+color.Reset, l)
		scraper.saveRepoList(m, l)
	}
	if len(a) == 0 {
		fmt.Println(color.Red + "[-] No archived repository found" + color.Reset)
	}

	fmt.Println(color.Green+"\nCompleted in "+color.Reset, time.Since(timeStart))
}
