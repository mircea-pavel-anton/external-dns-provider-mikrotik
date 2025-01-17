package mikrotik

import (
	"testing"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

func TestGetProviderSpecific(t *testing.T) {
	tests := []struct {
		name          string
		endpoint      *endpoint.Endpoint
		property      string
		expectedValue string
	}{
		{
			name: "Direct property exists",
			endpoint: &endpoint.Endpoint{
				ProviderSpecific: endpoint.ProviderSpecific{
					{Name: "comment", Value: "direct-comment"},
				},
			},
			property:      "comment",
			expectedValue: "direct-comment",
		},
		{
			name: "Prefixed property exists",
			endpoint: &endpoint.Endpoint{
				ProviderSpecific: endpoint.ProviderSpecific{
					{Name: "webhook/comment", Value: "prefixed-comment"},
				},
			},
			property:      "comment",
			expectedValue: "prefixed-comment",
		},
		{
			name: "Both properties exist - direct takes precedence",
			endpoint: &endpoint.Endpoint{
				ProviderSpecific: endpoint.ProviderSpecific{
					{Name: "comment", Value: "direct-comment"},
					{Name: "webhook/comment", Value: "prefixed-comment"},
				},
			},
			property:      "comment",
			expectedValue: "direct-comment",
		},
		{
			name: "Neither property exists",
			endpoint: &endpoint.Endpoint{
				ProviderSpecific: endpoint.ProviderSpecific{},
			},
			property:      "comment",
			expectedValue: "",
		},
		{
			name: "Weong key selected",
			endpoint: &endpoint.Endpoint{
				ProviderSpecific: endpoint.ProviderSpecific{
					{Name: "comment", Value: "direct-comment"},
				},
			},
			property:      "address-list",
			expectedValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := getProviderSpecific(tt.endpoint, tt.property)
			if value != tt.expectedValue {
				t.Errorf("Expected %q, got %q", tt.expectedValue, value)
			}
		})
	}
}

func TestIsEndpointMatching(t *testing.T) {
	tests := []struct {
		name          string
		endpointA     *endpoint.Endpoint
		endpointB     *endpoint.Endpoint
		expectedMatch bool
	}{
		// MATCHING CASES
		{
			name: "Matching basic properties",
			endpointA: &endpoint.Endpoint{
				DNSName:   "example.com",
				Targets:   endpoint.NewTargets("192.0.2.1"),
				RecordTTL: endpoint.TTL(3600),
			},
			endpointB: &endpoint.Endpoint{
				DNSName:   "example.com",
				Targets:   endpoint.NewTargets("192.0.2.1"),
				RecordTTL: endpoint.TTL(3600),
			},
			expectedMatch: true,
		},
		{
			name: "Matching provider-specific",
			endpointA: &endpoint.Endpoint{
				DNSName:   "example.com",
				Targets:   endpoint.NewTargets("192.0.2.1"),
				RecordTTL: endpoint.TTL(3600),
				ProviderSpecific: endpoint.ProviderSpecific{
					{Name: "comment", Value: "match"},
					{Name: "match-subdomain", Value: "true"},
					{Name: "address-list", Value: "default"},
					{Name: "regexp", Value: ".*"},
				},
			},
			endpointB: &endpoint.Endpoint{
				DNSName:   "example.com",
				Targets:   endpoint.NewTargets("192.0.2.1"),
				RecordTTL: endpoint.TTL(3600),
				ProviderSpecific: endpoint.ProviderSpecific{
					{Name: "webhook/comment", Value: "match"},
					{Name: "webhook/match-subdomain", Value: "true"},
					{Name: "webhook/address-list", Value: "default"},
					{Name: "webhook/regexp", Value: ".*"},
				},
			},
			expectedMatch: true,
		},

		// EDGE CASES
		{
			name: "Match-Subdomain: 'false' and unspecified should match",
			endpointA: &endpoint.Endpoint{
				DNSName:   "example.com",
				Targets:   endpoint.NewTargets("192.0.2.1"),
				RecordTTL: endpoint.TTL(3600),
				ProviderSpecific: endpoint.ProviderSpecific{
					{Name: "match-subdomain", Value: "false"},
				},
			},
			endpointB: &endpoint.Endpoint{
				DNSName:          "example.com",
				Targets:          endpoint.NewTargets("192.0.2.1"),
				RecordTTL:        endpoint.TTL(3600),
				ProviderSpecific: endpoint.ProviderSpecific{}, // unspecified match-subdomain
			},
			expectedMatch: true,
		},
		{
			name: "Disabled: 'false' and unspecified should match",
			endpointA: &endpoint.Endpoint{
				DNSName:   "example.com",
				Targets:   endpoint.NewTargets("192.0.2.1"),
				RecordTTL: endpoint.TTL(3600),
				ProviderSpecific: endpoint.ProviderSpecific{
					{Name: "disabled", Value: "false"},
				},
			},
			endpointB: &endpoint.Endpoint{
				DNSName:          "example.com",
				Targets:          endpoint.NewTargets("192.0.2.1"),
				RecordTTL:        endpoint.TTL(3600),
				ProviderSpecific: endpoint.ProviderSpecific{}, // unspecified disabled
			},
			expectedMatch: true,
		},

		// MISMATCH CASES
		{
			name: "Provider-specific properties do not match",
			endpointA: &endpoint.Endpoint{
				DNSName:   "example.com",
				Targets:   endpoint.NewTargets("192.0.2.1"),
				RecordTTL: endpoint.TTL(3600),
				ProviderSpecific: endpoint.ProviderSpecific{
					{Name: "comment", Value: "mismatch"},
				},
			},
			endpointB: &endpoint.Endpoint{
				DNSName:   "example.com",
				Targets:   endpoint.NewTargets("192.0.2.1"),
				RecordTTL: endpoint.TTL(3600),
				ProviderSpecific: endpoint.ProviderSpecific{
					{Name: "webhook/comment", Value: "different"},
				},
			},
			expectedMatch: false,
		},
		{
			name: "Mismatch in DNSName",
			endpointA: &endpoint.Endpoint{
				DNSName:   "example1.com",
				Targets:   endpoint.NewTargets("192.0.2.1"),
				RecordTTL: endpoint.TTL(3600),
			},
			endpointB: &endpoint.Endpoint{
				DNSName:   "example2.com",
				Targets:   endpoint.NewTargets("192.0.2.1"),
				RecordTTL: endpoint.TTL(3600),
			},
			expectedMatch: false,
		},
		{
			name: "Mismatch in Target",
			endpointA: &endpoint.Endpoint{
				DNSName:   "example.com",
				Targets:   endpoint.NewTargets("192.0.2.1"),
				RecordTTL: endpoint.TTL(3600),
			},
			endpointB: &endpoint.Endpoint{
				DNSName:   "example.com",
				Targets:   endpoint.NewTargets("192.0.2.2"),
				RecordTTL: endpoint.TTL(3600),
			},
			expectedMatch: false,
		},
		{
			name: "Mismatch in TTL",
			endpointA: &endpoint.Endpoint{
				DNSName:   "example.com",
				Targets:   endpoint.NewTargets("192.0.2.1"),
				RecordTTL: endpoint.TTL(3600),
			},
			endpointB: &endpoint.Endpoint{
				DNSName:   "example.com",
				Targets:   endpoint.NewTargets("192.0.2.1"),
				RecordTTL: endpoint.TTL(3601),
			},
			expectedMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := isEndpointMatching(tt.endpointA, tt.endpointB)
			if match != tt.expectedMatch {
				t.Errorf("Expected %v, got %v", tt.expectedMatch, match)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name          string
		haystack      []*endpoint.Endpoint
		needle        *endpoint.Endpoint
		expectContain bool
	}{
		{
			name: "Needle exists in haystack",
			haystack: []*endpoint.Endpoint{
				{
					DNSName:   "example1.com",
					Targets:   endpoint.NewTargets("192.2.2.1"),
					RecordTTL: endpoint.TTL(36),
				},
				{
					DNSName:   "example.com",
					Targets:   endpoint.NewTargets("192.0.2.1"),
					RecordTTL: endpoint.TTL(3600),
					ProviderSpecific: endpoint.ProviderSpecific{
						{Name: "comment", Value: "test"},
					},
				},
				{
					DNSName:   "example2.com",
					Targets:   endpoint.NewTargets("192.1.2.1"),
					RecordTTL: endpoint.TTL(360),
				},
			},
			needle: &endpoint.Endpoint{
				DNSName:   "example.com",
				Targets:   endpoint.NewTargets("192.0.2.1"),
				RecordTTL: endpoint.TTL(3600),
				ProviderSpecific: endpoint.ProviderSpecific{
					{Name: "webhook/comment", Value: "test"},
				},
			},
			expectContain: true,
		},
		{
			name: "Needle does not exist in haystack",
			haystack: []*endpoint.Endpoint{
				{
					DNSName:   "example1.com",
					Targets:   endpoint.NewTargets("192.0.2.1"),
					RecordTTL: endpoint.TTL(3600),
				},
				{
					DNSName:   "example2.com",
					Targets:   endpoint.NewTargets("192.0.2.1"),
					RecordTTL: endpoint.TTL(3600),
				},
				{
					DNSName:   "example3.com",
					Targets:   endpoint.NewTargets("192.0.2.1"),
					RecordTTL: endpoint.TTL(3600),
				},
			},
			needle: &endpoint.Endpoint{
				DNSName:   "example.org",
				Targets:   endpoint.NewTargets("192.0.2.1"),
				RecordTTL: endpoint.TTL(3600),
			},
			expectContain: false,
		},
		{
			name:     "Haystack is empty",
			haystack: []*endpoint.Endpoint{},
			needle: &endpoint.Endpoint{
				DNSName:   "example.com",
				Targets:   endpoint.NewTargets("192.0.2.1"),
				RecordTTL: endpoint.TTL(3600),
			},
			expectContain: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contains := contains(tt.haystack, tt.needle)
			if contains != tt.expectContain {
				t.Errorf("Expected %v, got %v", tt.expectContain, contains)
			}
		})
	}
}

func TestChanges(t *testing.T) {
	mikrotikProvider := &MikrotikProvider{
		client: &MikrotikApiClient{
			&MikrotikDefaults{},
			nil,
			nil,
		},
	}

	tests := []struct {
		name            string
		provider        *MikrotikProvider
		inputChanges    *plan.Changes
		expectedChanges *plan.Changes
	}{
		{
			name:     "Multiple matching records - all should be cleaned up",
			provider: mikrotikProvider,
			inputChanges: &plan.Changes{
				UpdateOld: []*endpoint.Endpoint{
					{
						DNSName:   "example.com",
						Targets:   endpoint.NewTargets("1.1.1.1"),
						RecordTTL: endpoint.TTL(3600),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "comment", Value: "test comment"},
							{Name: "address-list", Value: "main"},
							{Name: "match-subdomain", Value: ".*"},
						},
					},
					{
						DNSName:   "example.org",
						Targets:   endpoint.NewTargets("2.2.2.2"),
						RecordTTL: endpoint.TTL(300),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "comment", Value: "another comment"},
							{Name: "address-list", Value: "secondary"},
							{Name: "match-subdomain", Value: "*.example.com"},
						},
					},
				},
				UpdateNew: []*endpoint.Endpoint{
					{
						DNSName:   "example.com",
						Targets:   endpoint.NewTargets("1.1.1.1"),
						RecordTTL: endpoint.TTL(3600),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "webhook/comment", Value: "test comment"},
							{Name: "address-list", Value: "main"},
							{Name: "match-subdomain", Value: ".*"},
						},
					},
					{
						DNSName:   "example.org",
						Targets:   endpoint.NewTargets("2.2.2.2"),
						RecordTTL: endpoint.TTL(300),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "webhook/comment", Value: "another comment"},
							{Name: "address-list", Value: "secondary"},
							{Name: "match-subdomain", Value: "*.example.com"},
						},
					},
				},
			},
			expectedChanges: &plan.Changes{},
		},
		{
			name:     "Some matching, some different - only partial cleanup",
			provider: mikrotikProvider,
			inputChanges: &plan.Changes{
				UpdateOld: []*endpoint.Endpoint{
					{
						DNSName:   "matching.com",
						Targets:   endpoint.NewTargets("1.1.1.1"),
						RecordTTL: endpoint.TTL(3600),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "comment", Value: "old comment"},
						},
					},
					{
						DNSName:   "different.org",
						Targets:   endpoint.NewTargets("2.2.2.2"),
						RecordTTL: endpoint.TTL(300),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "comment", Value: "old comment"},
						},
					},
				},
				UpdateNew: []*endpoint.Endpoint{
					{
						DNSName:   "matching.com",
						Targets:   endpoint.NewTargets("1.1.1.1"),
						RecordTTL: endpoint.TTL(3600),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "webhook/comment", Value: "old comment"},
						},
					},
					{
						DNSName:   "different.org",
						Targets:   endpoint.NewTargets("2.2.2.2"),
						RecordTTL: endpoint.TTL(300),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "webhook/comment", Value: "new comment"},
						},
					},
				},
			},
			expectedChanges: &plan.Changes{
				UpdateOld: []*endpoint.Endpoint{
					{
						DNSName:   "different.org",
						Targets:   endpoint.NewTargets("2.2.2.2"),
						RecordTTL: endpoint.TTL(300),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "comment", Value: "old comment"},
						},
					},
				},
				UpdateNew: []*endpoint.Endpoint{
					{
						DNSName:   "different.org",
						Targets:   endpoint.NewTargets("2.2.2.2"),
						RecordTTL: endpoint.TTL(300),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "comment", Value: "new comment"},
						},
					},
				},
			},
		},
		{
			name:     "Different comments across multiple records - no cleanup",
			provider: mikrotikProvider,
			inputChanges: &plan.Changes{
				UpdateOld: []*endpoint.Endpoint{
					{
						DNSName:   "example.com",
						Targets:   endpoint.NewTargets("1.1.1.1"),
						RecordTTL: endpoint.TTL(3600),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "comment", Value: "old comment"},
						},
					},
					{
						DNSName:   "example.net",
						Targets:   endpoint.NewTargets("3.3.3.3"),
						RecordTTL: endpoint.TTL(120),
					},
				},
				UpdateNew: []*endpoint.Endpoint{
					{
						DNSName:   "example.com",
						Targets:   endpoint.NewTargets("1.1.1.1"),
						RecordTTL: endpoint.TTL(3600),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "webhook/comment", Value: "new comment"},
						},
					},
					{
						DNSName:   "example.net",
						Targets:   endpoint.NewTargets("3.3.3.3"),
						RecordTTL: endpoint.TTL(120),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "comment", Value: "new comment"},
						},
					},
				},
			},
			expectedChanges: &plan.Changes{
				UpdateOld: []*endpoint.Endpoint{
					{
						DNSName:   "example.com",
						Targets:   endpoint.NewTargets("1.1.1.1"),
						RecordTTL: endpoint.TTL(3600),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "comment", Value: "old comment"},
						},
					},
					{
						DNSName:   "example.net",
						Targets:   endpoint.NewTargets("3.3.3.3"),
						RecordTTL: endpoint.TTL(120),
					},
				},
				UpdateNew: []*endpoint.Endpoint{
					{
						DNSName:   "example.com",
						Targets:   endpoint.NewTargets("1.1.1.1"),
						RecordTTL: endpoint.TTL(3600),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "webhook/comment", Value: "new comment"},
						},
					},
					{
						DNSName:   "example.net",
						Targets:   endpoint.NewTargets("3.3.3.3"),
						RecordTTL: endpoint.TTL(120),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "comment", Value: "new comment"},
						},
					},
				},
			},
		},
		{
			name:     "Create record with zero value in TTL",
			provider: mikrotikProvider,
			inputChanges: &plan.Changes{
				Create: []*endpoint.Endpoint{
					{
						DNSName:   "example.org",
						Targets:   endpoint.NewTargets("2.2.2.2"),
						RecordTTL: endpoint.TTL(0),
						ProviderSpecific: endpoint.ProviderSpecific{
							{Name: "comment", Value: "another comment"},
							{Name: "address-list", Value: "secondary"},
							{Name: "match-subdomain", Value: "*.example.com"},
						},
					},
				},
			},
			expectedChanges: &plan.Changes{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputChanges := tt.provider.changes(tt.inputChanges)

			if len(outputChanges.UpdateOld) != len(tt.expectedChanges.UpdateOld) {
				t.Errorf("Expected UpdateOld length %d, got %d", len(tt.expectedChanges.UpdateOld), len(outputChanges.UpdateOld))
			}
			if len(outputChanges.UpdateNew) != len(tt.expectedChanges.UpdateNew) {
				t.Errorf("Expected UpdateNew length %d, got %d", len(tt.expectedChanges.UpdateNew), len(outputChanges.UpdateNew))
			}

			for i := range tt.expectedChanges.UpdateOld {
				if !isEndpointMatching(outputChanges.UpdateOld[i], tt.expectedChanges.UpdateOld[i]) {
					t.Errorf("Expected endpoint: %v , got %v", tt.expectedChanges.UpdateOld[i], outputChanges.UpdateOld[i])
				}
			}

			for i := range tt.expectedChanges.UpdateNew {
				if !isEndpointMatching(outputChanges.UpdateNew[i], tt.expectedChanges.UpdateNew[i]) {
					t.Errorf("Expected endpoint: %v , got %v", tt.expectedChanges.UpdateNew[i], outputChanges.UpdateNew[i])
				}
			}
			for i := range outputChanges.Create {
				if outputChanges.Create[i].RecordTTL != 0 {
					t.Errorf("Expected Create endpoint TTL %d, got %d", 0, outputChanges.Create[i].RecordTTL)
				}
			}
		})
	}
}
