package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gocarina/gocsv"
)

const MAX_APPS_PER_CALL = 50

type AppSearchResponse struct {
	Results []AppSearchResult `json:"results"`
}
type AppSearchResult struct {
	Genre            string  `json:"genre" csv:"genre"`
	GenreID          string  `json:"genreId" csv:"genreId"`
	AppID            string  `json:"appId" csv:"appId"`
	Title            string  `json:"title" csv:"title"`
	Summary          string  `json:"summary" csv:"summary"`
	DeveloperEmail   string  `json:"developerEmail" csv:"developerEmail"`
	DeveloperWebsite string  `json:"developerWebsite" csv:"developerWebsite"`
	DeveloperAddress string  `json:"developerAddress" csv:"developerAddress"`
	MinInstalls      int     `json:"minInstalls" csv:"minInstalls"`
	MaxInstalls      int     `json:"maxInstalls" csv:"maxInstalls"`
	Score            float64 `json:"score" csv:"score"`
	Updated          string  `json:"updated" csv:"updated"`
	AdSupported      bool    `json:"adSupported" csv:"adSupported"`
	Price            string  `json:"price" csv:"price"`
	OffersIAP        bool    `json:"offersIAP" csv:"offersIAP"`
}

var inputFileName, outputFileName, host, runType, topCategory string
var numOfApps int

func init() {
	flag.StringVar(&runType, "t", "", "type of run: [search, top]")
	flag.StringVar(&inputFileName, "i", "terms.txt", "a file with (search terms or categories [based on https://github.com/facundoolano/google-play-scraper/blob/dev/lib/constants.js#L3]) to process")
	flag.StringVar(&outputFileName, "o", "results.csv", "a csv that will contain the results")
	flag.StringVar(&host, "h", "localhost:3000", "the url of the host running google-play-scraper")
	flag.IntVar(&numOfApps, "n", 5, "the number of apps to return from each search")
}

func main() {
	flag.Parse()

	if runType != "top" && runType != "search" {
		flag.Usage()
		return
	}

	apps, err := getApps()
	if err != nil {
		fmt.Printf("Get apps err: %v", err)
		return
	}

	if len(apps) > 0 {

		outFile, err := os.OpenFile(outputFileName, os.O_RDWR|os.O_CREATE, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
		defer outFile.Close()

		err = gocsv.MarshalFile(&apps, outFile) // Use this to save the CSV back to the file
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Written output to [%s]\n", outputFileName)
	}
}

func getApps() ([]AppSearchResult, error) {

	appsHash := make(map[string]bool)
	apps := []AppSearchResult{}

	var endpoint = url.URL{
		Scheme: "http",
		Path:   "/api/apps",
		Host:   host,
	}

	fmt.Printf("processing file [%s]\n", inputFileName)

	f, err := os.Open(inputFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	r := bufio.NewReader(f)
	line, err := r.ReadString(10) // line defined once
	for err != io.EOF {

		if len(strings.TrimSpace(line)) != 0 {

			start := 0
			num := min(numOfApps, MAX_APPS_PER_CALL)
			v := url.Values{
				"fullDetail": {"true"},
				"start":      {strconv.Itoa(start)},
				"num":        {strconv.Itoa(num)},
			}

			for start < numOfApps {
				if runType == "search" {
					v.Set("q", strings.TrimSpace(line))
				} else {
					v.Set("category", strings.TrimSpace(line))
				}

				resp := &AppSearchResponse{}
				if err = httpGetJSON(endpoint, v, resp); err != nil {
					return nil, err
				}

				for _, currApp := range resp.Results {
					if _, ok := appsHash[currApp.AppID]; !ok {
						appsHash[currApp.AppID] = true
						upd, err := convertDate(currApp.Updated)
						if err != nil {
							fmt.Printf("convertDate [%s] err: %v", currApp.Updated, err)
						} else {
							currApp.Updated = upd
						}
						apps = append(apps, currApp)
					}
				}

				start += num
				num = min(numOfApps-start, MAX_APPS_PER_CALL)
				v.Set("start", strconv.Itoa(start))
				v.Set("num", strconv.Itoa(num))
			}
		}

		line, err = r.ReadString(10) //  line was defined before
	}
	return apps, nil
}

func httpGetJSON(u url.URL, v url.Values, target interface{}) error {

	retry := 0

	u.RawQuery = v.Encode()
	fmt.Printf("calling google play api [%s]\n", u.String())
	r, err := http.Get(u.String())
	for httpError := isAPIError(r, err); httpError != nil; {
		if retry > 3 {
			return err
		}
		if err != nil {
			fmt.Printf("http.Get error [%s] ... retrying in 10 seconds\n", err.Error())
		} else {
			fmt.Printf("http.Get response code [%d] ... retrying in 10 seconds\n", r.StatusCode)
		}
		time.Sleep(10 * time.Second)
		retry++
		r, err = http.Get(u.String())
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

func isAPIError(r *http.Response, err error) error {
	if err != nil {
		return err
	}
	if r.StatusCode != 200 {
		return fmt.Errorf("HTTP Resonse Status[%s]", r.Status)
	}
	return nil
}

var longMonthNames = []string{
	"---",
	"January",
	"February",
	"March",
	"April",
	"May",
	"June",
	"July",
	"August",
	"September",
	"October",
	"November",
	"December",
}

func convertDate(strDate string) (string, error) {
	month := 0
	s := strings.Split(strDate, " ")

	//Find month
	for i := range longMonthNames {
		if longMonthNames[i] == s[0] {
			month = i
			break
		}
	}
	if month == 0 {
		return "", errors.New("Month invalid")
	}
	x := len(s[1])
	day, err := strconv.Atoi(s[1][:x-1])
	if err != nil {
		return "", err
	}
	year, err := strconv.Atoi(s[2])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%.4d-%.2d-%.2d", year, month, day), nil
}

func min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}
