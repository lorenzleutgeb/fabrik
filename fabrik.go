package main

import (
	"errors"
	"flag"
	"fmt"
	"html"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// Where the menu is posted.
	target = "http://www.diefabrik.co.at/mittagsmenue/index.html"

	// Layout of dates on website.
	dateLayout = "02.01.2006"

	// If this string is found, assume holidays.
	holidayMagic = "urlaub"
)

var (
	// Time of execution
	now = time.Now()

	// The day of the week as a shorthand (derived from now in init)
	wd time.Weekday

	force = flag.Bool("force", false, "always download, no caching.")

	errFabrikClosed   = errors.New("fabrik is closed")
	errMenuFromPast   = errors.New("the menu is outdated")
	errMenuFromFuture = errors.New("the menu is from the future")
	errFabrikResting  = errors.New("fabrik is on a day off")
	errFabrikHoliday  = errors.New("fabrik is probably on holiday, fall back to manual check")
)

func init() {
	flag.Parse()
	wd = now.Weekday()
}

func bail(err error) {
	fmt.Println(err)
	os.Exit(1)
}

func main() {
	if wd == time.Saturday || wd == time.Sunday {
		bail(errFabrikClosed)
	}

	var meal string
	var err error

	if !*force {
		meal, err = readTemp()

		if err != nil && err != os.ErrNotExist {
			bail(err)
		}
	}

	if meal == "" {
		body, err := fetch()
		if err != nil {
			bail(err)
		}

		if checkHoliday(body) {
			bail(errFabrikHoliday)
		}

		meal, err = extractMeal(body)
		if err != nil {
			bail(err)
		}

		if meal != "" {
			writeTemp(meal)
		}
	}

	fmt.Println(meal)
}

func fetch() (string, error) {
	res, err := http.Get(target)

	if err != nil {
		return "", err
	}

	b, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()

	if err != nil {
		return "", err
	}

	return string(b), nil
}

func checkHoliday(body string) bool {
	return strings.Contains(strings.ToLower(body), holidayMagic)
}

func extractMeal(body string) (meal string, err error) {
	from, to, err := extractValidity(body)

	if err != nil {
		return
	} else if now.After(to) {
		return "", errMenuFromPast
	} else if now.Before(from) {
		return "", errMenuFromFuture
	}

	idx := (wd - 1) * 2
	day := strconv.Itoa(int(idx))

	if idx == 8 {
		day = "last"
	}

	re := regexp.MustCompile(`(?s)<tr class="tr-even tr-` + day + `">.+?<td class="td-2">(.+?)</td>`)
	matches := re.FindStringSubmatch(body)

	meal = html.UnescapeString(matches[1])

	if meal == "Ruhetag" {
		return "", errFabrikResting
	}

	return
}

// extractValidity looks for a date range in the given HTML.
func extractValidity(body string) (from, to time.Time, err error) {
	re := regexp.MustCompile(`<h2>(?P<from>\d{2}\.\d{2}.\d{4}) bis (?P<to>\d{2}\.\d{2}.\d{4})</h2>`)
	matches := re.FindStringSubmatch(body)

	from, err = parseDate(matches[1])
	if err != nil {
		return
	}

	to, err = parseDate(matches[2])
	to = to.Add(24 * time.Hour)
	return
}

// parseDate takes a string and tries to parse it
// with the constant dateLayout.
func parseDate(raw string) (time.Time, error) {
	return time.Parse(dateLayout, raw)
}

// writeTemp writes the given string to a file in the
// default temporary directory.
func writeTemp(meal string) error {
	file, err := ioutil.TempFile("", "fabrik-")
	if err != nil {
		return err
	}
	defer file.Close()
	file.WriteString(meal)
	return nil
}

// readTemp looks for a temporary file that was placed there
// by this tool in the default temporary directory of the OS
// and that was modified today.
// If no such file is found, it will return os.ErrNotExist.
// Otherwise it will attempt to read the file (retuning any
// error if that fails) and return it's contents.
func readTemp() (meal string, err error) {
	matches, err := filepath.Glob(path.Join(os.TempDir(), "fabrik-*"))

	// filepath.Glob should only return err if
	// the glob pattern is malformed
	if err != nil {
		panic(err)
	}

	// filepath.Glob did not find any matches
	if matches == nil {
		return "", os.ErrNotExist
	}

	filename := ""

	// truncate to 00:00 of today
	from := now.Truncate(24 * time.Hour)

	for _, v := range matches {
		stat, err := os.Stat(v)
		mtime := stat.ModTime()

		if err != nil {
			continue
		} else if mtime.After(from) {
			filename = v
			break
		}
	}

	// no file last modified today, so again
	// we did not find what we wanted
	if filename == "" {
		return "", os.ErrNotExist
	}

	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	return string(b), nil
}
