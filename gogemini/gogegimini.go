package gogemini

import (
	"log"

	"github.com/Allan-Nava/go-gemini/configuration"
	"github.com/Allan-Nava/go-gemini/constants"
	"github.com/go-resty/resty/v2"
)

type IGoGemini interface {
	// api
	GetAnswer() error
	//
}

type gemini struct {
	configuration *configuration.Configuration
	restClient    *resty.Client
	sNlM0e        string
}

func NewGoGemini(configuration *configuration.Configuration) IGoGemini {
	g := &gemini{
		configuration: configuration,
	}
	g.restClient = resty.New()
	g.restClient.SetBaseURL(constants.BASE_URL)
	/*
		g.restClient.SetHeader("Referer", "https://bard.google.com/")
		//"Host": "bard.google.com",
		//"X-Same-Domain": "1",
		//"User-Agent": "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36",
		//"Content-Type": "application/x-www-form-urlencoded;charset=UTF-8",
		//"Origin": "https://bard.google.com",
		//"Referer": "https://bard.google.com/",
	*/
	g.restClient.SetHeader("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	g.restClient.SetHeader("Host", "bard.google.com")
	g.restClient.SetHeader("X-Same-Domain", "1")
	g.restClient.SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36")
	g.restClient.SetHeader("Origin", "https://bard.google.com")
	if configuration.IsDebug {
		g.restClient.SetDebug(true)
	}
	return g
}

// Resty Methods

func (o *gemini) restyPost(url string, body interface{}) (*resty.Response, error) {
	resp, err := o.restClient.R().
		SetHeader("Accept", "application/json").
		SetBody(body).
		Post(url)

	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (o *gemini) restyGet(url string, queryParams map[string]string) (*resty.Response, error) {
	resp, err := o.restClient.R().
		SetQueryParams(queryParams).
		Get(url)
	//
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// internal api
// _get_snim0e
func (o *gemini) getSnim0e() error {
	resp, err := o.restyGet("/", nil)
	if err != nil {
		return err
	}
	log.Println("resp ", resp)
	// re.search(r"SNlM0e\":\"(.*?)\"", resp.text).group(1)
	return nil
}
