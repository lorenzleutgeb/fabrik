package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

const (
	// Where the menu is posted
	target = "http://www.diefabrik.co.at/mittagsmenue/index.html"

	// Layout of dates on website
	dateLayout = "02.01.2006"
)

var (
	// Time of execution
	now = time.Now()

	// The day of the week as a shorthand (derived from now in init)
	wd time.Weekday

	force = flag.Bool("force", false, "always download, no caching.")

	ErrFabrikClosed   = errors.New("fabrik is closed")
	ErrMenuFromPast   = errors.New("the menu is outdated")
	ErrMenuFromFuture = errors.New("the menu is from the future")
)

func init() {
	flag.Parse()
	wd = now.Weekday()
}

func main() {
	if wd == time.Saturday || wd == time.Sunday {
		fmt.Fprintln(os.Stderr, "ERROR: Today is "+wd.String()+". Try again on Monday.")
		os.Exit(1)
		return
	}

	var meal string

	if !*force {
		meal = readTemp()
	}

	if meal == "" {
		body, _ := fetch()
		meal = extractMeal(body)

		if meal != "" {
			writeTemp(meal)
		}
	} else {
		fmt.Println(meal)
	}
}

func fetch() (body string, err error) {
	res, err := http.Get(target)

	if err != nil {
		return
	}

	b, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()

	if err != nil {
		return
	}

	return string(b), nil
}

func extractMeal(body string) (meal string) {

	from, to, err := extractValidity(body)

	if err != nil {
		fmt.Fprintln(os.Stderr, "WARNING: Failed to interpret validity for the menu. Assuming it is valid; result could be wrong.")
	} else if now.After(to) {
		fmt.Fprintln(os.Stderr, "ERROR: The menu is outdated, aborting.")
		os.Exit(2)

	} else if now.Before(from) {
		fmt.Fprintln(os.Stderr, "WARNING: The menu is from the future. Results may be confusing.")
	}

	idx := (wd - 1) * 2
	day := strconv.Itoa(int(idx))

	if idx == 8 {
		day = "last"
	}

	re := regexp.MustCompile(`(?s)<tr class="tr-even tr-` + day + `">.+?<td class="td-2">(.+?)</td>`)
	matches := re.FindStringSubmatch(body)

	meal = matches[1]

	if meal == "Ruhetag" {
		fmt.Fprintln(os.Stderr, "ERROR: Fabrik is closed today.")
		os.Exit(3)
	}

	fmt.Println(meal)
	return
}

func extractValidity(body string) (from, to time.Time, err error) {
	re := regexp.MustCompile(`<h2>(?P<from>\d{2}\.\d{2}.\d{4}) bis (?P<to>\d{2}\.\d{2}.\d{4})</h2>`)
	matches := re.FindStringSubmatch(body)

	from, err = parseDate(matches[1])
	if err != nil {
		return
	}

	to, err = parseDate(matches[2])
	return
}

func parseDate(raw string) (time.Time, error) {
	return time.Parse(dateLayout, raw)
}

func writeTemp(meal string) {
	file, _ := ioutil.TempFile("", "fabrik-")
	defer file.Close()
	file.WriteString(meal)
}

func readTemp() (meal string) {
	matches, _ := filepath.Glob(path.Join(os.TempDir(), "fabrik-*"))
	if matches == nil {
		return
	}

	var filename string
	from := now.Truncate(24 * time.Hour)

	for _, name := range matches {
		stat, err := os.Stat(name)
		mtime := stat.ModTime()

		if err != nil {
			continue
		} else if mtime.After(from) {
			filename = name
			break
		}
	}

	if filename == "" {
		return
	}

	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}
	return string(b)
}
