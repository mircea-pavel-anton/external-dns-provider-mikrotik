// Rest API Docs: https://help.mikrotik.com/docs/display/ROS/REST+API

package mikrotik

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/publicsuffix"
	"sigs.k8s.io/external-dns/endpoint"
)

type MikrotikDefaults struct {
	TTL int64 `env:"MIKROTIK_DEFAULT_TTL" envDefault:"3600"`
}

// MikrotikConnectionConfig holds the connection details for the API client
type MikrotikConnectionConfig struct {
	BaseUrl       string `env:"MIKROTIK_BASEURL,notEmpty"`
	Username      string `env:"MIKROTIK_USERNAME,notEmpty"`
	Password      string `env:"MIKROTIK_PASSWORD,notEmpty"`
	SkipTLSVerify bool   `env:"MIKROTIK_SKIP_TLS_VERIFY" envDefault:"false"`
}

// MikrotikApiClient encapsulates the client configuration and HTTP client
type MikrotikApiClient struct {
	*MikrotikDefaults
	*MikrotikConnectionConfig
	*http.Client
}

// MikrotikSystemInfo represents MikroTik system information
// https://help.mikrotik.com/docs/display/ROS/Resource
type MikrotikSystemInfo struct {
	ArchitectureName     string `json:"architecture-name"`
	BadBlocks            string `json:"bad-blocks"`
	BoardName            string `json:"board-name"`
	BuildTime            string `json:"build-time"`
	CPU                  string `json:"cpu"`
	CPUCount             string `json:"cpu-count"`
	CPUFrequency         string `json:"cpu-frequency"`
	CPULoad              string `json:"cpu-load"`
	FactorySoftware      string `json:"factory-software"`
	FreeHDDSpace         string `json:"free-hdd-space"`
	FreeMemory           string `json:"free-memory"`
	Platform             string `json:"platform"`
	TotalHDDSpace        string `json:"total-hdd-space"`
	TotalMemory          string `json:"total-memory"`
	Uptime               string `json:"uptime"`
	Version              string `json:"version"`
	WriteSectSinceReboot string `json:"write-sect-since-reboot"`
	WriteSectTotal       string `json:"write-sect-total"`
}

// NewMikrotikClient creates a new instance of MikrotikApiClient
func NewMikrotikClient(config *MikrotikConnectionConfig, defaults *MikrotikDefaults) (*MikrotikApiClient, error) {
	log.Infof("creating a new Mikrotik API Client")

	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		log.Errorf("failed to create cookie jar: %v", err)
		return nil, err
	}

	client := &MikrotikApiClient{
		MikrotikDefaults:         defaults,
		MikrotikConnectionConfig: config,
		Client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: config.SkipTLSVerify,
				},
			},
			Jar: jar,
		},
	}

	return client, nil
}

// GetSystemInfo fetches system information from the MikroTik API
func (c *MikrotikApiClient) GetSystemInfo() (*MikrotikSystemInfo, error) {
	log.Debugf("fetching system information.")

	// Send the request
	resp, err := c.doRequest(http.MethodGet, "system/resource", nil)
	if err != nil {
		log.Errorf("error fetching system info: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Parse the response
	var info MikrotikSystemInfo
	if err = json.NewDecoder(resp.Body).Decode(&info); err != nil {
		log.Errorf("error decoding response body: %v", err)
		return nil, err
	}
	log.Debugf("got system info: %+v", info)

	return &info, nil
}

// CreateDNSRecord sends a request to create a new DNS record
func (c *MikrotikApiClient) CreateDNSRecord(endpoint *endpoint.Endpoint) (*DNSRecord, error) {
	log.Infof("creating DNS record: %+v", endpoint)

	// Convert ExternalDNS to Mikrotik DNS
	record, err := NewDNSRecord(endpoint)
	if err != nil {
		log.Errorf("error converting ExternalDNS endpoint to Mikrotik DNS Record: %v", err)
		return nil, err
	}

	// Serialize the data to JSON to be sent to the API
	jsonBody, err := json.Marshal(record)
	if err != nil {
		log.Errorf("error marshalling DNS record: %v", err)
		return nil, err
	}

	// Send the request
	resp, err := c.doRequest(http.MethodPut, "ip/dns/static", bytes.NewReader(jsonBody))
	if err != nil {
		log.Errorf("error creating DNS record: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Parse the response
	if err = json.NewDecoder(resp.Body).Decode(&record); err != nil {
		log.Errorf("Error decoding response body: %v", err)
		return nil, err
	}
	log.Infof("Created record: %+v", record)

	return record, nil
}

// GetAllDNSRecords fetches all DNS records from the MikroTik API
func (c *MikrotikApiClient) GetAllDNSRecords() ([]DNSRecord, error) {
	log.Infof("fetching all DNS records")

	// Send the request
	resp, err := c.doRequest(http.MethodGet, "ip/dns/static?type=A,AAAA,CNAME,TXT,MX,SRV,NS", nil)
	if err != nil {
		log.Errorf("error fetching DNS records: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Parse the response
	var records []DNSRecord
	if err = json.NewDecoder(resp.Body).Decode(&records); err != nil {
		log.Errorf("error decoding response body: %v", err)
		return nil, err
	}
	log.Debugf("fetched %d DNS records: %v", len(records), records)

	return records, nil
}

// DeleteDNSRecord sends a request to delete a DNS record
func (c *MikrotikApiClient) DeleteDNSRecord(endpoint *endpoint.Endpoint) error {
	log.Infof("deleting DNS record: %+v", endpoint)

	// Send the request
	record, err := c.lookupDNSRecord(endpoint.DNSName, endpoint.RecordType)
	if err != nil {
		log.Errorf("failed lookup for DNS record: %+v", err)
		return err
	}

	// Parse the response
	resp, err := c.doRequest(http.MethodDelete, fmt.Sprintf("ip/dns/static/%s", record.ID), nil)
	if err != nil {
		log.Errorf("error deleting DNS record: %+v", err)
		return err
	}
	defer resp.Body.Close()
	log.Infof("record deleted")

	return nil
}

// lookupDNSRecord searches for a DNS record by key and type
func (c *MikrotikApiClient) lookupDNSRecord(key, recordType string) (*DNSRecord, error) {
	log.Infof("Searching for DNS record: Key: %s, RecordType: %s", key, recordType)

	searchParams := fmt.Sprintf("name=%s", key)
	if recordType != "A" {
		searchParams = fmt.Sprintf("%s&type=%s", searchParams, recordType)
	}
	log.Debugf("Search params: %s", searchParams)

	// Send the request
	resp, err := c.doRequest(http.MethodGet, fmt.Sprintf("ip/dns/static?%s", searchParams), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse the response
	var record []DNSRecord
	if err = json.NewDecoder(resp.Body).Decode(&record); err != nil {
		log.Errorf("Error decoding response body: %v", err)
		return nil, err
	}
	if len(record) == 0 {
		return nil, errors.New("record list is empty")
	}

	log.Infof("Found record: %+v", record)

	return &record[0], nil
}

// doRequest sends an HTTP request to the MikroTik API with credentials
func (c *MikrotikApiClient) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	endpoint_url := fmt.Sprintf("%s/rest/%s", c.MikrotikConnectionConfig.BaseUrl, path)
	log.Debugf("sending %s request to: %s", method, endpoint_url)

	req, err := http.NewRequest(method, endpoint_url, body)
	if err != nil {
		log.Errorf("failed to create HTTP request: %v", err)
		return nil, err
	}

	req.SetBasicAuth(c.MikrotikConnectionConfig.Username, c.MikrotikConnectionConfig.Password)

	resp, err := c.Client.Do(req)
	if err != nil {
		log.Errorf("error sending HTTP request: %v", err)
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Errorf("request failed with status %s, response: %s", resp.Status, string(respBody))
		return nil, fmt.Errorf("request failed: %s", resp.Status)
	}
	log.Debugf("request succeeded with status %s", resp.Status)

	return resp, nil
}
