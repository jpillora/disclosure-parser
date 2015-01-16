package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/agnivade/levenshtein"
	"github.com/otiai10/gosseract"
)

var targets = map[string]int{
	"Constitution (disclosures by Members) Regulation 1983": 25,
	"SECTION 2—MEMBER'S ORDINARY RETURN":                    10,
}

func worker(work chan string, matched map[string]bool, wg *sync.WaitGroup) {
	// Decreasing internal counter for wait-group as soon as goroutine finishes
	defer wg.Done()

	for path := range work {
		if match(path) {
			matched[path] = true
		}
	}
}

func match(path string) bool {

	txt := gosseract.Must(map[string]string{"src": path})

	lines := strings.Split(txt, "\n")

	dmin := 1000

	for i, line := range lines {
		line = strings.Trim(line, " ")
		//skip first line and empty strings
		if i == 0 || line == "" {
			continue
		}

		for target, threshold := range targets {
			d, _ := levenshtein.ComputeDistance(line, target)
			if d < dmin {
				dmin = d
			}
			if d < threshold {
				// fmt.Printf("%s [%5d] '%s'\n", path, d, line)
				return true
			}
		}
	}
	// fmt.Printf("%s NO MATCH [%5d]\n", path, dmin)
	return false
}

func main() {

	paths := getPaths()

	matched := map[string]bool{}
	work := make(chan string)
	wg := new(sync.WaitGroup)

	// Adding CPU many workers
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go worker(work, matched, wg)
	}

	// Spreading paths to "free" goroutines
	for _, path := range paths {
		work <- path
	}

	// Kill the workers (break u)
	close(work)
	wg.Wait()

	// Loop through paths and place them
	// into their matched directory
	form := 0
	page := 1
	for _, path := range paths {

		if matched[path] {
			form++
			page = 1
		}

		dir := "output/form" + strconv.Itoa(form) + "/"
		if err := os.MkdirAll(dir, 0777); err != nil {
			log.Fatal(err)
		}

		dst := dir + "page-" + strconv.Itoa(page) + ".png"

		fmt.Printf("copying %s -> %s\n", path, dst)

		copyFile(dst, path)
		page++
	}
}

func getPaths() []string {
	files, err := ioutil.ReadDir("input")
	if err != nil {
		log.Fatal("missing 'input' dir")
	}
	paths := make([]string, 0)

	matcher := regexp.MustCompile(`\.png$`)

	for _, f := range files {
		p := "input/" + f.Name()
		if matcher.Match([]byte(p)) {
			paths = append(paths, p)
		}
	}
	return paths
}

func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	cerr := out.Close()
	if err != nil {
		return err
	}
	return cerr
}
