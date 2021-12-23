package packetframe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

const (
	mediaType      = "application/json"
	defaultBaseURL = "https://packetframe.com/api/"
)

func (c *packetframeProvider) fetchDomainList() error {
	c.domainIndex = map[string]domain{}
	dr := &domainResponse{}
	endpoint := "zones/list"
	if err := c.get(endpoint, dr); err != nil {
		return fmt.Errorf("failed fetching domain list (Packetframe): %s", err)
	}
	for _, domain := range dr.Message {
		c.domainIndex[domain.Zone] = domain
		// log.Printf("%s zone detected", domain.Zone)
	}

	return nil
}

func (c *packetframeProvider) createRecord(zoneName string, rec *domainRecord) (*domainRecord, error) {
	log.Println("MADE DOMAIN")
	endpoint := fmt.Sprintf("zone/%s/add", zoneName)

	req, err := c.newRequest(http.MethodPost, endpoint, rec)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		fmt.Println("ERROR")
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrors(resp)
	}

	record := &domainRecord{}

	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(record); err != nil {
		return nil, err
	}

	return record, nil
}

func (c *packetframeProvider) modifyRecord(zoneName string, recordID int, rec *domainRecord) error {
	_, err := c.createRecord(zoneName, rec)
	if err != nil {
		return err
	}
	err = c.deleteRecord(zoneName, recordID)
	if err != nil {
		return err
	}
	return nil
}

func (c *packetframeProvider) deleteRecord(zoneName string, recordID int) error {
	endpoint := fmt.Sprintf("zone/%s/delete_record/%d", zoneName, recordID)
	req, err := c.newRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return c.handleErrors(resp)
	}

	return nil
}

func (c *packetframeProvider) newRequest(method, endpoint string, body interface{}) (*http.Request, error) {
	rel, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	u := c.baseURL.ResolveReference(rel)

	buf := new(bytes.Buffer)
	if body != nil {
		err = json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", mediaType)
	req.Header.Add("Accept", mediaType)
	return req, nil
}

func (c *packetframeProvider) get(endpoint string, target interface{}) error {
	req, err := c.newRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return c.handleErrors(resp)
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	return decoder.Decode(target)
}

func (c *packetframeProvider) handleErrors(resp *http.Response) error {
	defer resp.Body.Close()
	fmt.Println("ERROR")
	decoder := json.NewDecoder(resp.Body)

	errs := &errorResponse{}

	if err := decoder.Decode(errs); err != nil {
		return fmt.Errorf("bad status code from Packetframe: %d not 200. Failed to decode response", resp.StatusCode)
	}

	buf := bytes.NewBufferString(fmt.Sprintf("bad status code from Packetframe: %d not 200", resp.StatusCode))

	for _, err := range errs.Errors {
		buf.WriteString("\n- ")

		if err.Field != "" {
			buf.WriteString(err.Field)
			buf.WriteString(": ")
		}

		buf.WriteString(err.Reason)
	}

	return errors.New(buf.String())
}

type basicResponse struct {
	Results int `json:"results"`
	Pages   int `json:"pages"`
	Page    int `json:"page"`
}

type domain struct {
	Records []domainRecord `json:"records"`
	Serial  string         `json:"serial"`
	Type    string         `json:"type"`
	Users   []string       `json:"users"`
	Zone    string         `json:zone`
}

type domainResponse struct {
	Message []domain `json:"message"`
	Success bool     `json:"success"`
}

type recordResponse struct {
	basicResponse
	Data []domainRecord `json:"data"`
}

type domainRecord struct {
	Label   string `json:"label"`
	TTL     int    `json:"ttl"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	Proxied bool   `json:"proxied"`
}

type errorResponse struct {
	Errors []struct {
		Field  string `json:"field"`
		Reason string `json:"reason"`
	} `json:"errors"`
}
