package main

import ("net/http"
	"log"
	"sync")

func CheckHealthSite(addr string, resCh chan<- string) {
	resp, err := http.Get(addr)
	if err != nil {
		log.Printf("Error checking health of %s: %v\n", addr, err)
		resCh <- "not ok"
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("Site %s is healthy\n", addr)
		resCh <- "ok"
	} else {
		log.Printf("Site %s is unhealthy: %s\n", addr, resp.Status)
		resCh <- "not ok"
	}
}

func CheckHealth(sites []string, resCh chan string){
	var wg sync.WaitGroup
	for _, site := range sites {
		log.Printf("Checking health of %s\n", site)
		wg.Add(1)
		go func() {CheckHealthSite(site, resCh)
			wg.Done()}()
	}
	go func() {
	wg.Wait()
	close(resCh)}()
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("Starting health checker...")

	var sites = []string{
		"http://127.0.0.1",
		"http://example.com",}

	resCh := make(chan string, len(sites))

	CheckHealth(sites, resCh)

	for res := range resCh {
		if res == "ok" {
			log.Println("Site is healthy")
		} else {
			log.Println("Site is unhealthy")
		}
	}
}
