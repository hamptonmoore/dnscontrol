package packetframe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

const (
	mediaType      = "application/json"
	defaultBaseURL = "https://v4.packetframe.com/api/"
)

func (c *packetframeProvider) fetchDomainList() error {
	c.domainIndex = map[string]zone{}
	dr := &domainResponse{}
	endpoint := "dns/zones"
	if err := c.get(endpoint, dr); err != nil {
		return fmt.Errorf("failed fetching domain list (Packetframe): %s", err)
	}
	for _, zone := range dr.Data.Zones {
		c.domainIndex[zone.Zone] = zone
		log.Printf("%s zone detected", zone.Zone)
	}

	return nil
}

func (c *packetframeProvider) getRecords(zoneID string) ([]domainRecord, error) {
	var records []domainRecord
	dr := &recordResponse{}
	endpoint := fmt.Sprintf("dns/records/%s", zoneID)
	if err := c.get(endpoint, dr); err != nil {
		return records, fmt.Errorf("failed fetching domain list (Packetframe): %s", err)
	}
	for _, record := range dr.Data.Records {
		records = append(records, record)
	}

	return records, nil
}

func (c *packetframeProvider) createRecord(zoneID string, rec *domainRecord) (*domainRecord, error) {
	log.Println("MADE DOMAIN RECORD")
	endpoint := fmt.Sprintf("dns/records")

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

// func (c *packetframeProvider) modifyRecord(zoneName string, recordID int, rec *domainRecord) error {
// 	_, err := c.createRecord(zoneName, rec)
// 	if err != nil {
// 		return err
// 	}
// 	err = c.deleteRecord(zoneName, recordID)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

func (c *packetframeProvider) deleteRecord(zoneID string, recordID string) error {
	endpoint := "dns/records"
	req, err := c.newRequest(http.MethodDelete, endpoint, deleteRequest{Zone: zoneID, Record: recordID})
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
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(string(body))
	return nil
}

type zone struct {
	ID         string   `json:"id"`
	Zone       string   `json:zone`
	Users      []string `json:"users"`
	UserEmails []string `json:"user_emails"`
}

type domainResponse struct {
	Data struct {
		Zones []zone `json:"zones"`
	} `json:"data"`
	Message string `json:"message"`
	Success bool   `json:"success"`
}

type deleteRequest struct {
	Record string `json:"record"`
	Zone   string `json:"zone"`
}

type recordResponse struct {
	Data struct {
		Records []domainRecord `json:"records"`
	} `json:"data"`
	Message string `json:"message"`
	Success bool   `json:"success"`
}

type domainRecord struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Label string `json:"label"`
	Value string `json:"value"`
	TTL   int    `json:"ttl"`
	Proxy bool   `json:"proxy"`
	Zone  string `json:zone`
}
