package main

import (
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

var (
	target = "http://www.diefabrik.co.at/mittagsmenue/index.html"
)

func main() {
	wd := time.Now().Weekday()

	if wd == time.Saturday || wd == time.Sunday {
		return
	}

	res, err := http.Get(target)

	if err != nil {
		panic(err)
	}

	body, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()

	if err != nil {
		panic(err)
	}

	idx := (wd - 1) * 2
	day := strconv.Itoa(int(idx))

	if idx == 8 {
		day = "last"
	}

	re := regexp.MustCompile(`(?s)<tr class="tr-even tr-` + day + `">.+?<td class="td-2">(.+?)</td>`)
	m := re.FindSubmatch(body)

	print(string(m[1]) + "\n")
}
