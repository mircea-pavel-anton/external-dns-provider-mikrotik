package mikrotik

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

// DNS Provider for working with mikrotik
type MikrotikProvider struct {
	provider.BaseProvider

	client       *MikrotikApiClient
	domainFilter endpoint.DomainFilter
}

// NewMikrotikProvider initializes a new DNSProvider, of the Mikrotik variety
func NewMikrotikProvider(domainFilter endpoint.DomainFilter, defaults *MikrotikDefaults, config *MikrotikConnectionConfig) (provider.Provider, error) {
	// Create the Mikrotik API Client
	client, err := NewMikrotikClient(config, defaults)
	if err != nil {
		return nil, fmt.Errorf("failed to create the MikroTik client: %w", err)
	}

	// Ensure the Client can connect to the API by fetching system info
	info, err := client.GetSystemInfo()
	if err != nil {
		log.Errorf("failed to connect to the MikroTik RouterOS API Endpoint: %v", err)
		return nil, err
	}
	log.Infof("connected to board %s running RouterOS version %s (%s)", info.BoardName, info.Version, info.ArchitectureName)

	// If the client connects properly, create the DNS Provider
	p := &MikrotikProvider{
		client:       client,
		domainFilter: domainFilter,
	}

	return p, nil
}

// Records returns the list of all DNS records.
func (p *MikrotikProvider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	records, err := p.client.GetAllDNSRecords()
	if err != nil {
		return nil, err
	}

	var endpoints []*endpoint.Endpoint
	for _, record := range records {
		ep, err := record.toExternalDNSEndpoint()
		if err != nil {
			log.Warnf("Failed to convert mikrotik record to external-dns endpoint: %+v", err)
			continue
		}

		if !p.domainFilter.Match(ep.DNSName) {
			continue
		}

		endpoints = append(endpoints, ep)
	}

	return endpoints, nil
}

// ApplyChanges applies a given set of changes in the DNS provider.
func (p *MikrotikProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	changes = p.changes(changes)

	for _, endpoint := range append(changes.UpdateOld, changes.Delete...) {
		if err := p.client.DeleteDNSRecord(endpoint); err != nil {
			return err
		}
	}

	for _, endpoint := range append(changes.Create, changes.UpdateNew...) {
		if _, err := p.client.CreateDNSRecord(endpoint); err != nil {
			return err
		}
	}

	return nil
}

// GetDomainFilter returns the domain filter for the provider.
func (p *MikrotikProvider) GetDomainFilter() endpoint.DomainFilterInterface {
	return p.domainFilter
}

// ================================================================================================
// UTILS
// ================================================================================================
func getProviderSpecific(ep *endpoint.Endpoint, ps string) string {
	value, valueExists := ep.GetProviderSpecificProperty(ps)
	if !valueExists {
		value, _ = ep.GetProviderSpecificProperty(fmt.Sprintf("webhook/%s", ps))
	}
	return value
}

func isEndpointMatching(a *endpoint.Endpoint, b *endpoint.Endpoint) bool {
	if a.DNSName != b.DNSName || a.Targets[0] != b.Targets[0] || a.RecordTTL != b.RecordTTL {
		return false
	}

	aComment := getProviderSpecific(a, "comment")
	bComment := getProviderSpecific(b, "comment")
	if aComment != bComment {
		return false
	}

	aMatchSubdomain := getProviderSpecific(a, "match-subdomain")
	if aMatchSubdomain == "" {
		aMatchSubdomain = "false"
	}
	bMatchSubdomain := getProviderSpecific(b, "match-subdomain")
	if bMatchSubdomain == "" {
		bMatchSubdomain = "false"
	}
	if aMatchSubdomain != bMatchSubdomain {
		return false
	}

	aDisabled := getProviderSpecific(a, "disabled")
	if aDisabled == "" {
		aDisabled = "false"
	}
	bDisabled := getProviderSpecific(b, "disabled")
	if bDisabled == "" {
		bDisabled = "false"
	}
	if aDisabled != bDisabled {
		return false
	}

	aAddressList := getProviderSpecific(a, "address-list")
	bAddressList := getProviderSpecific(b, "address-list")
	if aAddressList != bAddressList {
		return false
	}

	aRegexp := getProviderSpecific(a, "regexp")
	bRegexp := getProviderSpecific(b, "regexp")
	return aRegexp == bRegexp
}

func contains(haystack []*endpoint.Endpoint, needle *endpoint.Endpoint) bool {
	for _, v := range haystack {
		if isEndpointMatching(needle, v) {
			return true
		}
	}
	return false
}

func (p *MikrotikProvider) changes(changes *plan.Changes) *plan.Changes {
	// Initialize new plan -> we don't really need to worry about Create or Delete changes.
	// Only updates are sketchy
	newChanges := &plan.Changes{
		Create:    changes.Create,
		Delete:    changes.Delete,
		UpdateOld: []*endpoint.Endpoint{},
		UpdateNew: []*endpoint.Endpoint{},
	}

	duplicates := []*endpoint.Endpoint{}

	for _, create := range changes.Create {
		if !create.RecordTTL.IsConfigured() {
			create.RecordTTL = endpoint.TTL(p.client.TTL)
			newChanges.Create = append(newChanges.Create, create)
		}
	}

	for _, old := range changes.UpdateOld {
		for _, new := range changes.UpdateNew {
			if isEndpointMatching(old, new) {
				duplicates = append(duplicates, old)
			}
		}
	}

	for _, old := range changes.UpdateOld {
		if !contains(duplicates, old) {
			newChanges.UpdateOld = append(newChanges.UpdateOld, old)
		}
	}
	for _, new := range changes.UpdateNew {
		if !contains(duplicates, new) {
			newChanges.UpdateNew = append(newChanges.UpdateNew, new)
		}
	}

	return newChanges
}
