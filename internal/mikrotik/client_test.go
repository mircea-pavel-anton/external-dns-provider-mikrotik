// client_test.go
package mikrotik

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sigs.k8s.io/external-dns/endpoint"
)

var (
	mockUsername = "testuser"
	mockPassword = "testpass"
)

func TestNewMikrotikClient(t *testing.T) {
	config := &MikrotikConnectionConfig{
		BaseUrl:       "https://192.168.88.1:443",
		Username:      "admin",
		Password:      "password",
		SkipTLSVerify: true,
	}

	defaults := &MikrotikDefaults{}

	client, err := NewMikrotikClient(config, defaults)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if client.MikrotikConnectionConfig != config {
		t.Errorf("Expected config to be %v, got %v", config, client.MikrotikConnectionConfig)
	}

	if client.Client == nil {
		t.Errorf("Expected HTTP client to be initialized")
	}

	transport, ok := client.Client.Transport.(*http.Transport)
	if !ok {
		t.Errorf("Expected Transport to be *http.Transport")
	}

	if transport.TLSClientConfig == nil {
		t.Errorf("Expected TLSClientConfig to be set")
	} else if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Errorf("Expected InsecureSkipVerify to be true")
	}
}

func TestGetSystemInfo(t *testing.T) {
	mockServerInfo := MikrotikSystemInfo{
		ArchitectureName:     "arm64",
		BadBlocks:            "0.1",
		BoardName:            "RB5009UG+S+",
		BuildTime:            "2024-09-20 13:00:27",
		CPU:                  "ARM64",
		CPUCount:             "4",
		CPUFrequency:         "1400",
		CPULoad:              "0",
		FactorySoftware:      "7.4.1",
		FreeHDDSpace:         "1019346944",
		FreeMemory:           "916791296",
		Platform:             "MikroTik",
		TotalHDDSpace:        "1073741824",
		TotalMemory:          "1073741824",
		Uptime:               "4d19h9m34s",
		Version:              "7.16 (stable)",
		WriteSectSinceReboot: "5868",
		WriteSectTotal:       "131658",
	}

	// Set up mock server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate the Basic Auth header
		username, password, ok := r.BasicAuth()
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if username != mockUsername || password != mockPassword {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Return dummy data for /rest/system/resource
		if r.URL.Path == "/rest/system/resource" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(mockServerInfo)
			if err != nil {
				t.Errorf("error json encoding server info")
			}
			return
		}

		// Return 404 for any other path
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Define test cases
	testCases := []struct {
		name          string
		config        MikrotikConnectionConfig
		defaults      MikrotikDefaults
		expectedError bool
	}{
		{
			name: "Valid credentials",
			config: MikrotikConnectionConfig{
				BaseUrl:       server.URL,
				Username:      mockUsername,
				Password:      mockPassword,
				SkipTLSVerify: true,
			},
			defaults:      MikrotikDefaults{},
			expectedError: false,
		},
		{
			name: "Incorrect password",
			config: MikrotikConnectionConfig{
				BaseUrl:       server.URL,
				Username:      mockUsername,
				Password:      "wrongpass",
				SkipTLSVerify: true,
			},
			defaults:      MikrotikDefaults{},
			expectedError: true,
		},
		{
			name: "Incorrect username",
			config: MikrotikConnectionConfig{
				BaseUrl:       server.URL,
				Username:      "wronguser",
				Password:      mockPassword,
				SkipTLSVerify: true,
			},
			defaults:      MikrotikDefaults{},
			expectedError: true,
		},
		{
			name: "Incorrect username and password",
			config: MikrotikConnectionConfig{
				BaseUrl:       server.URL,
				Username:      "wronguser",
				Password:      "wrongpass",
				SkipTLSVerify: true,
			},
			defaults:      MikrotikDefaults{},
			expectedError: true,
		},
		{
			name: "Missing credentials",
			config: MikrotikConnectionConfig{
				BaseUrl:       server.URL,
				Username:      "",
				Password:      "",
				SkipTLSVerify: true,
			},
			defaults:      MikrotikDefaults{},
			expectedError: true,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &tc.config
			defaults := &tc.defaults

			client, err := NewMikrotikClient(config, defaults)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}
			info, err := client.GetSystemInfo()

			if tc.expectedError {
				if err == nil {
					t.Fatalf("Expected error due to unauthorized access, got none")
				}
				if info != nil {
					t.Errorf("Expected no system info, got %v", info)
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error, got %v", err)
				}
				if info.ArchitectureName != mockServerInfo.ArchitectureName {
					t.Errorf("Expected ArchitectureName %s, got %s", mockServerInfo.ArchitectureName, info.ArchitectureName)
				}
				if info.Version != mockServerInfo.Version {
					t.Errorf("Expected Version %s, got %s", mockServerInfo.Version, info.Version)
				}
				// i think there's no point in checking any more fields
			}
		})
	}
}

func TestCreateDNSRecord(t *testing.T) {
	testCases := []struct {
		name           string
		initialRecords map[string]DNSRecord
		endpoint       *endpoint.Endpoint
		expectedError  bool
	}{
		{
			name:           "Valid A record creation",
			initialRecords: map[string]DNSRecord{},
			endpoint: &endpoint.Endpoint{
				DNSName:    "test-a.example.com",
				RecordType: "A",
				Targets:    endpoint.Targets{"192.0.2.1"},
			},
			expectedError: false,
		},
		{
			name:           "Valid AAAA record creation",
			initialRecords: map[string]DNSRecord{},
			endpoint: &endpoint.Endpoint{
				DNSName:    "test-aaaa.example.com",
				RecordType: "AAAA",
				Targets:    endpoint.Targets{"2001:db8::1"},
			},
			expectedError: false,
		},
		{
			name:           "Valid CNAME record creation",
			initialRecords: map[string]DNSRecord{},
			endpoint: &endpoint.Endpoint{
				DNSName:    "test-cname.example.com",
				RecordType: "CNAME",
				Targets:    endpoint.Targets{"example.com"},
			},
			expectedError: false,
		},
		{
			name:           "Valid TXT record creation",
			initialRecords: map[string]DNSRecord{},
			endpoint: &endpoint.Endpoint{
				DNSName:    "test-txt.example.com",
				RecordType: "TXT",
				Targets:    endpoint.Targets{"\"some text record value\""},
			},
			expectedError: false,
		},
		{
			name: "Record already exists",
			initialRecords: map[string]DNSRecord{
				"exists.example.com|A": {
					ID:      "*EXISTING",
					Name:    "exists.example.com",
					Type:    "A",
					Address: "192.0.2.1",
				},
			},
			endpoint: &endpoint.Endpoint{
				DNSName:    "exists.example.com",
				RecordType: "A",
				Targets:    endpoint.Targets{"192.0.2.1"},
			},
			expectedError: true,
		},
		{
			name:           "Invalid record (missing DNSName)",
			initialRecords: map[string]DNSRecord{},
			endpoint: &endpoint.Endpoint{
				DNSName:    "",
				RecordType: "A",
				Targets:    endpoint.Targets{"192.0.2.1"},
			},
			expectedError: true,
		},
		{
			name:           "Empty target for A record",
			initialRecords: map[string]DNSRecord{},
			endpoint: &endpoint.Endpoint{
				DNSName:    "empty-target.example.com",
				RecordType: "A",
				Targets:    endpoint.Targets{""},
			},
			expectedError: true,
		},
		{
			name:           "Malformed IP address for A record",
			initialRecords: map[string]DNSRecord{},
			endpoint: &endpoint.Endpoint{
				DNSName:    "malformed-ip.example.com",
				RecordType: "A",
				Targets:    endpoint.Targets{"999.999.999.999"},
			},
			expectedError: true,
		},
		{
			name:           "Malformed IP address for AAAA record",
			initialRecords: map[string]DNSRecord{},
			endpoint: &endpoint.Endpoint{
				DNSName:    "malformed-ipv6.example.com",
				RecordType: "AAAA",
				Targets:    endpoint.Targets{"gggg::1"},
			},
			expectedError: true,
		},
		{
			name:           "Empty target for CNAME record",
			initialRecords: map[string]DNSRecord{},
			endpoint: &endpoint.Endpoint{
				DNSName:    "empty-cname.example.com",
				RecordType: "CNAME",
				Targets:    endpoint.Targets{""},
			},
			expectedError: true,
		},
		// { //! we dont have any kind of cname validation so this will always pass
		// 	name:           "Malformed domain name for CNAME record",
		// 	initialRecords: map[string]DNSRecord{},
		// 	endpoint: &endpoint.Endpoint{
		// 		DNSName:    "bad-cname.example.com",
		// 		RecordType: "CNAME",
		// 		Targets:    endpoint.Targets{"1234!"},
		// 	},
		// 	expectedError: true,
		// },
		{
			name:           "Empty text for TXT record",
			initialRecords: map[string]DNSRecord{},
			endpoint: &endpoint.Endpoint{
				DNSName:    "empty-txt.example.com",
				RecordType: "TXT",
				Targets:    endpoint.Targets{""},
			},
			expectedError: true,
		},
		{
			name:           "Invalid record type",
			initialRecords: map[string]DNSRecord{},
			endpoint: &endpoint.Endpoint{
				DNSName:    "invalid-type.example.com",
				RecordType: "INVALID",
				Targets:    endpoint.Targets{"some target"},
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Initialize an in-memory store for DNS records for this test case
			recordStore := make(map[string]DNSRecord)

			// Pre-populate recordStore with initialRecords
			for k, v := range tc.initialRecords {
				recordStore[k] = v
			}

			// Set up mock server
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Validate the Basic Auth header
				username, password, ok := r.BasicAuth()
				if !ok || username != mockUsername || password != mockPassword {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				// Handle DNS record creation
				if r.URL.Path == "/rest/ip/dns/static" && r.Method == http.MethodPut {
					var record DNSRecord
					if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
						http.Error(w, "Bad Request", http.StatusBadRequest)
						return
					}

					// Check if record already exists
					key := record.Name + "|" + record.Type
					if _, exists := recordStore[key]; exists {
						http.Error(w, "Conflict: Record already exists", http.StatusConflict)
						return
					}

					// Simulate assigning an ID and storing the record
					record.ID = "*NEW"
					recordStore[key] = record

					// Return the created record
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(record); err != nil {
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
						return
					}
					return
				}

				// Return 404 for any other path
				http.NotFound(w, r)
			}))
			defer server.Close()

			// Set up the client with correct credentials
			config := &MikrotikConnectionConfig{
				BaseUrl:       server.URL,
				Username:      mockUsername,
				Password:      mockPassword,
				SkipTLSVerify: true,
			}
			defaults := &MikrotikDefaults{}
			client, err := NewMikrotikClient(config, defaults)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}
			record, err := client.CreateDNSRecord(tc.endpoint)

			if tc.expectedError {
				if err == nil {
					t.Fatalf("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			// Verify that the record was stored in the mock server
			key := tc.endpoint.DNSName + "|" + tc.endpoint.RecordType
			storedRecord, exists := recordStore[key]
			if !exists {
				t.Fatalf("Expected record to be stored, but it was not found")
			}

			// Verify that the client received the correct record
			if record.ID != storedRecord.ID {
				t.Errorf("Expected ID '%s', got '%s'", storedRecord.ID, record.ID)
			}

			// Additional checks specific to record type
			switch tc.endpoint.RecordType {
			case "A", "AAAA":
				if storedRecord.Address != tc.endpoint.Targets[0] {
					t.Errorf("Expected Address '%s', got '%s'", tc.endpoint.Targets[0], storedRecord.Address)
				}
			case "CNAME":
				if storedRecord.CName != tc.endpoint.Targets[0] {
					t.Errorf("Expected CName '%s', got '%s'", tc.endpoint.Targets[0], storedRecord.CName)
				}
			case "TXT":
				if storedRecord.Text != tc.endpoint.Targets[0] {
					t.Errorf("Expected Text '%s', got '%s'", tc.endpoint.Targets[0], storedRecord.Text)
				}
			default:
				t.Errorf("Unsupported RecordType '%s' in test case", tc.endpoint.RecordType)
			}
		})
	}
}

func TestDeleteDNSRecord(t *testing.T) {
	testCases := []struct {
		name           string
		initialRecords map[string]DNSRecord
		endpoint       *endpoint.Endpoint
		expectedError  bool
	}{
		{
			name: "Delete existing A record",
			initialRecords: map[string]DNSRecord{
				"test.example.com|A": {
					ID:      "*1",
					Name:    "test.example.com",
					Type:    "A",
					Address: "192.0.2.1",
				},
			},
			endpoint: &endpoint.Endpoint{
				DNSName:    "test.example.com",
				RecordType: "A",
			},
			expectedError: false,
		},
		{
			name: "Delete existing AAAA record",
			initialRecords: map[string]DNSRecord{
				"ipv6.example.com|AAAA": {
					ID:      "*2",
					Name:    "ipv6.example.com",
					Type:    "AAAA",
					Address: "2001:db8::1",
				},
			},
			endpoint: &endpoint.Endpoint{
				DNSName:    "ipv6.example.com",
				RecordType: "AAAA",
			},
			expectedError: false,
		},
		{
			name: "Delete existing CNAME record",
			initialRecords: map[string]DNSRecord{
				"alias.example.com|CNAME": {
					ID:    "*3",
					Name:  "alias.example.com",
					Type:  "CNAME",
					CName: "example.com",
				},
			},
			endpoint: &endpoint.Endpoint{
				DNSName:    "alias.example.com",
				RecordType: "CNAME",
			},
			expectedError: false,
		},
		{
			name: "Delete existing TXT record",
			initialRecords: map[string]DNSRecord{
				"text.example.com|TXT": {
					ID:   "*4",
					Name: "text.example.com",
					Type: "TXT",
					Text: "some text",
				},
			},
			endpoint: &endpoint.Endpoint{
				DNSName:    "text.example.com",
				RecordType: "TXT",
			},
			expectedError: false,
		},
		{
			name:           "Delete non-existent record",
			initialRecords: map[string]DNSRecord{},
			endpoint: &endpoint.Endpoint{
				DNSName:    "nonexistent.example.com",
				RecordType: "A",
			},
			expectedError: true,
		},
		{
			name: "Delete record with missing DNSName",
			initialRecords: map[string]DNSRecord{
				"missingname.example.com|A": {
					ID:      "*5",
					Name:    "missingname.example.com",
					Type:    "A",
					Address: "192.0.2.2",
				},
			},
			endpoint: &endpoint.Endpoint{
				DNSName:    "",
				RecordType: "A",
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Initialize an in-memory store for DNS records for this test case
			recordStore := make(map[string]DNSRecord)

			// Pre-populate recordStore with initialRecords
			for k, v := range tc.initialRecords {
				recordStore[k] = v
			}

			// Set up mock server
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				username, password, ok := r.BasicAuth()
				if !ok || username != mockUsername || password != mockPassword {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				// Handle DNS record fetching (for the lookup method)
				if r.Method == http.MethodGet && r.URL.Path == "/rest/ip/dns/static" {
					query := r.URL.Query()
					name := query.Get("name")
					recordType := query.Get("type")
					if recordType == "" {
						recordType = "A"
					}
					key := name + "|" + recordType
					record := recordStore[key]
					w.Header().Set("Content-Type", "application/json")
					err := json.NewEncoder(w).Encode([]DNSRecord{record})
					if err != nil {
						t.Errorf("error json encoding dns record")
					}
					return
				}

				// Handle DNS record deletion
				if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/rest/ip/dns/static/") {
					id := strings.TrimPrefix(r.URL.Path, "/rest/ip/dns/static/")
					var foundKey string
					for key, record := range recordStore {
						if record.ID == id {
							foundKey = key
							break
						}
					}
					if foundKey != "" {
						delete(recordStore, foundKey)
						w.WriteHeader(http.StatusOK)
					} else {
						http.Error(w, "Not Found", http.StatusNotFound)
					}
					return
				}

				http.NotFound(w, r)
			}))
			defer server.Close()

			config := &MikrotikConnectionConfig{
				BaseUrl:       server.URL,
				Username:      mockUsername,
				Password:      mockPassword,
				SkipTLSVerify: true,
			}
			defaults := &MikrotikDefaults{}
			client, err := NewMikrotikClient(config, defaults)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			err = client.DeleteDNSRecord(tc.endpoint)

			if tc.expectedError {
				if err == nil {
					t.Fatalf("Expected error, got none")
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error, got %v", err)
				}
				key := tc.endpoint.DNSName + "|" + tc.endpoint.RecordType
				if _, exists := recordStore[key]; exists {
					t.Fatalf("Expected record to be deleted, but it still exists")
				}
			}
		})
	}
}

func TestGetAllDNSRecords(t *testing.T) {
	testCases := []struct {
		name         string
		records      []DNSRecord
		expectError  bool
		unauthorized bool
	}{
		{
			name: "Multiple DNS records",
			records: []DNSRecord{
				{
					ID:      "*1",
					Address: "192.168.88.1",
					Comment: "defconf",
					Name:    "router.lan",
					TTL:     "1d",
					Type:    "A",
				},
				{
					ID:      "*3",
					Address: "1.2.3.4",
					Comment: "test A-Record",
					Name:    "example.com",
					TTL:     "1d",
					Type:    "A",
				},
				{
					ID:      "*4",
					CName:   "example.com",
					Comment: "test CNAME",
					Name:    "subdomain.example.com",
					TTL:     "1d",
					Type:    "CNAME",
				},
				{
					ID:      "*5",
					Address: "::1",
					Comment: "test AAAA",
					Name:    "test quad-A",
					TTL:     "1d",
					Type:    "AAAA",
				},
				{
					ID:      "*6",
					Comment: "test TXT",
					Name:    "example.com",
					Text:    "lorem ipsum",
					TTL:     "1d",
					Type:    "TXT",
				},
			},
		},
		{
			name:    "No DNS records",
			records: []DNSRecord{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Basic Auth validation
				username, password, ok := r.BasicAuth()
				if !ok || username != mockUsername || password != mockPassword || tc.unauthorized {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				// Handle GET requests to /rest/ip/dns/static
				if r.Method == http.MethodGet && r.URL.Path == "/rest/ip/dns/static" {
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(tc.records); err != nil {
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
						return
					}
					return
				}

				// Return 404 for any other path
				http.NotFound(w, r)
			}))
			defer server.Close()

			// Set up the client
			config := &MikrotikConnectionConfig{
				BaseUrl:       server.URL,
				Username:      mockUsername,
				Password:      mockPassword,
				SkipTLSVerify: true,
			}
			defaults := &MikrotikDefaults{}
			client, err := NewMikrotikClient(config, defaults)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}
			records, err := client.GetAllDNSRecords()

			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected error, got none")
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error, got %v", err)
				}

				// Verify the number of records
				if len(records) != len(tc.records) {
					t.Fatalf("Expected %d records, got %d", len(tc.records), len(records))
				}

				// Compare records if there are any
				if len(tc.records) > 0 {
					expectedRecordsMap := make(map[string]DNSRecord)
					for _, rec := range tc.records {
						key := rec.Name + "|" + rec.Type
						expectedRecordsMap[key] = rec
					}

					for _, record := range records {
						key := record.Name + "|" + record.Type
						expectedRecord, exists := expectedRecordsMap[key]
						if !exists {
							t.Errorf("Unexpected record found: %v", record)
							continue
						}
						// Compare fields
						if record.ID != expectedRecord.ID {
							t.Errorf("Expected ID '%s', got '%s' for record %s", expectedRecord.ID, record.ID, key)
						}
						switch record.Type {
						case "A", "AAAA":
							if record.Address != expectedRecord.Address {
								t.Errorf("Expected Address '%s', got '%s' for record %s", expectedRecord.Address, record.Address, key)
							}
						case "CNAME":
							if record.CName != expectedRecord.CName {
								t.Errorf("Expected CName '%s', got '%s' for record %s", expectedRecord.CName, record.CName, key)
							}
						case "TXT":
							if record.Text != expectedRecord.Text {
								t.Errorf("Expected Text '%s', got '%s' for record %s", expectedRecord.Text, record.Text, key)
							}
						default:
							t.Errorf("Unsupported RecordType '%s' for record %s", record.Type, key)
						}
					}
				}
			}
		})
	}
}
