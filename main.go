package main

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// refs. https://docs.github.com/ko/rest/releases/releases?apiVersion=2022-11-28#get-a-release-by-tag-name
const repo = "bitflow-si/healthpilot-vue"

// const token = "Bearer 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" // Use OAUTH2 token
const token = "Bearer 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"
const api_release_list_url = "https://api.github.com/repos/%s/releases/latest"
const api_down_url = "https://api.github.com/repos/%s/releases/assets/%s"

const direct_url = "/%s/healthpilot.zip"

var download = true
var assets = true

func prepareRequest(url string) *http.Request {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("Authorization", token)
	return req
}

// e.g. gh api -H "Accept: application/vnd.github+json" -H "X-GitHub-Api-Version: 2022-11-28" "/repos/bitflow-si/healthpilot-vue/releases"
func hook(c *fiber.Ctx) error {

	// e.g. https://github.com/bitflow-si/healthpilot-vue/releases/download/v0.0.7/healthpilot.zip
	url := fmt.Sprintf(api_release_list_url, repo) // tag
	log.Println("Url", url)
	req := prepareRequest(url)
	// call github
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error while making request", err)
		var errMsg = fmt.Sprintf("{code: 500, msg: '%s'}", err)
		return c.Status(200).JSON(errMsg)
	}
	// status in <200 or >299
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		log.Printf("Error %s\n", resp.Status)
		var errMsg = fmt.Sprintf("{code: 500, msg: '%s'}", resp.Status)
		return c.Status(200).JSON(errMsg)
	}
	// res assets[0].url
	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading response", err)
		var errMsg = fmt.Sprintf("{code: 500, msg: '%s'}", err)
		return c.Status(200).JSON(errMsg)
	}

	// prepare result
	result := make(map[string]interface{})
	json.Unmarshal(bodyText, &result)

	// print download url
	results := make([]interface{}, 0)

	if !assets {
		// no assets info, just wanna direct download
		if download {
			results = append(results, result["id"])
		} else {
			results = append(results, result["zipball_url"])
		}
	} else {
		for _, asset := range result["assets"].([]interface{}) {
			if download {
				results = append(results, asset.(map[string]interface{})["id"])
			} else {
				results = append(results, asset.(map[string]interface{})["browser_download_url"])
			}
		}
	}

	if !download {
		// only print results
		// assets[0].url
		for _, res := range results {
			log.Printf("no down res %f", res)
		}
	} else {
		// Download results - parallel downloading, use channel to syncronize
		c := make(chan int)
		for _, res := range results {
			var assetId = strings.Split(fmt.Sprintf("%f", res), ".")[0]
			go downloadResource(assetId)
		}
		// wait for downloads end
		for i := 0; i < len(results); i++ {
			<-c
		}
	}

	return c.Status(200).JSON("{code: 200, msg: 'OK'}")
}

// clientid/secret
// 3b325f3619f21c4dd448:d14d4a9ffb42d6f0f753f3d2503325a5172d3fa9
// Download resource from given url, write 1 in chan when finished
func downloadResource(assetId string) {

	url := fmt.Sprintf(api_down_url, repo, assetId)
	log.Printf("Download: %s\n", url)
	req := prepareRequest(url)
	req.Header.Del("Accept")
	req.Header.Add("Accept", "application/octet-stream")

	client := http.Client{}
	resp, _ := client.Do(req)

	disp := resp.Header.Get("Content-disposition")
	re := regexp.MustCompile(`filename=(.+)`)
	matches := re.FindAllStringSubmatch(disp, -1)

	if len(matches) == 0 || len(matches[0]) == 0 {
		log.Println("WTF: ", matches)
		log.Println(resp.Header)
		log.Println(req)
		return
	}

	disp = matches[0][1]

	f, err := os.OpenFile(disp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0664)
	if err != nil {
		log.Fatal(err)
	}

	b := make([]byte, 4096)
	var i int

	for err == nil {
		i, err = resp.Body.Read(b)
		f.Write(b[:i])
	}
	fmt.Printf("Finished: %s -> %s\n", url, disp)
	f.Close()
}

func main() {

	app := fiber.New()

	app.Get("/rels/hook", func(c *fiber.Ctx) error {
		return hook(c)
	})

	log.Fatal(app.Listen(":3000"))

}
