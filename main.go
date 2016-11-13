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

const SEARCH_URL = "http://%s/api/apps/?fullDetail=true&num=%d&q=%s"

type AppSearchResponse struct {
	Results []AppSearchResult `json:"results"`
}
type AppSearchResult struct {
	Genre          string  `json:"genre" csv:"genre"`
	GenreID        string  `json:"genreId" csv:"genreId"`
	AppID          string  `json:"appId" csv:"appId"`
	Title          string  `json:"title" csv:"title"`
	Summary        string  `json:"summary" csv:"summary"`
	DeveloperEmail string  `json:"developerEmail" csv:"developerId"`
	MinInstalls    int     `json:"minInstalls" csv:"minInstalls"`
	MaxInstalls    int     `json:"maxInstalls" csv:"maxInstalls"`
	Score          float64 `json:"score" csv:"score"`
	Updated        string  `json:"updated" csv:"updated"`
	AdSupported    bool    `json:"adSupported" csv:"adSupported"`
	Price          string  `json:"price" csv:"price"`
	OffersIAP      bool    `json:"offersIAP" csv:"offersIAP"`
}

var inputFileName, outputFileName, host string
var numOfApps int

func init() {
	flag.StringVar(&inputFileName, "i", "terms.txt", "a file with search terms to process")
	flag.StringVar(&outputFileName, "o", "results.csv", "a csv that will contain the results")
	flag.StringVar(&host, "h", "localhost:3000", "the url of the host running google-play-scraper")
	flag.IntVar(&numOfApps, "n", 5, "the number of apps to return from each search")
}

func main() {
	flag.Parse()
	fmt.Printf("processing file [%s]\n", inputFileName)

	f, err := os.Open(inputFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	appsHash := make(map[string]bool)
	apps := []AppSearchResult{}

	r := bufio.NewReader(f)
	line, err := r.ReadString(10) // line defined once
	for err != io.EOF {

		url := fmt.Sprintf(SEARCH_URL, host, numOfApps, url.QueryEscape(line))

		resp := &AppSearchResponse{}
		if err = httpGetJSON(url, resp); err != nil {
			log.Fatal(err)
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

		line, err = r.ReadString(10) //  line was defined before
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

func httpGetJSON(url string, target interface{}) error {

	retry := 0

	r, err := http.Get(url)
	for httpError := isAPIError(r, err); httpError != nil; {
		if retry > 3 {
			return err
		}
		fmt.Printf("http.Get error [%s] ... retrying in 10 seconds\n", err.Error())
		time.Sleep(10 * time.Second)
		retry++
		r, err = http.Get(url)
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
