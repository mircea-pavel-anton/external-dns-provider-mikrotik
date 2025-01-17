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
// getProviderSpecific retrieves a provider-specific property from the endpoint, looking both values
// that could come from annotations (i.e. webhook/%s) as well as values from CRD (i.e. %s).
// If the property is not found, it returns the specified default value.
func (p *MikrotikProvider) getProviderSpecificOrDefault(ep *endpoint.Endpoint, ps string, defaultValue string) string {
	value, valueExists := ep.GetProviderSpecificProperty(ps)
	if valueExists {
		log.Debugf("Found provider-specific property '%s' with value: %s", ps, value)
		return value
	}

	log.Debugf("Provider-specific property '%s' not found, checking for webhook/%s", ps, ps)
	value, valueExists = ep.GetProviderSpecificProperty(fmt.Sprintf("webhook/%s", ps))
	if valueExists {
		log.Debugf("Found provider-specific property 'webhook/%s' with value: %s", ps, value)
		return value
	}

	log.Debugf("Property '%s' not found, returning default value: %s", ps, defaultValue)
	return defaultValue
}

// compareEndpoints compares two endpoints to determine if they are identical, keeping in mind empty/default states.
func (p *MikrotikProvider) compareEndpoints(a *endpoint.Endpoint, b *endpoint.Endpoint) bool {
	log.Debugf("Comparing endpoint a: %v", a)
	log.Debugf("Against endpoint b: %v", b)

	if a.DNSName != b.DNSName {
		log.Debugf("DNSName mismatch: %v != %v", a.DNSName, b.DNSName)
		return false
	}

	if a.Targets[0] != b.Targets[0] {
		log.Debugf("Targets[0] mismatch: %v != %v", a.Targets[0], b.Targets[0])
		return false
	}

	aRelevantTTL := a.RecordTTL != 0 && a.RecordTTL != endpoint.TTL(p.client.TTL)
	bRelevantTTL := b.RecordTTL != 0 && b.RecordTTL != endpoint.TTL(p.client.TTL)
	if a.RecordTTL != b.RecordTTL && (aRelevantTTL || bRelevantTTL) {
		log.Debugf("RecordTTL mismatch: %v != %v", a.RecordTTL, b.RecordTTL)
		return false
	}

	aComment := p.getProviderSpecificOrDefault(a, "comment", "")
	bComment := p.getProviderSpecificOrDefault(b, "comment", "")
	if aComment != bComment {
		log.Debugf("Comment mismatch: %v != %v", aComment, bComment)
		return false
	}

	aMatchSubdomain := p.getProviderSpecificOrDefault(a, "match-subdomain", "false")
	bMatchSubdomain := p.getProviderSpecificOrDefault(b, "match-subdomain", "false")
	if aMatchSubdomain != bMatchSubdomain {
		log.Debugf("MatchSubdomain mismatch: %v != %v", aMatchSubdomain, bMatchSubdomain)
		return false
	}

	aDisabled := p.getProviderSpecificOrDefault(a, "disabled", "false")
	bDisabled := p.getProviderSpecificOrDefault(b, "disabled", "false")
	if aDisabled != bDisabled {
		log.Debugf("Disabled mismatch: %v != %v", aDisabled, bDisabled)
		return false
	}

	aAddressList := p.getProviderSpecificOrDefault(a, "address-list", "")
	bAddressList := p.getProviderSpecificOrDefault(b, "address-list", "")
	if aAddressList != bAddressList {
		log.Debugf("AddressList mismatch: %v != %v", aAddressList, bAddressList)
		return false
	}

	aRegexp := p.getProviderSpecificOrDefault(a, "regexp", "")
	bRegexp := p.getProviderSpecificOrDefault(b, "regexp", "")
	if aRegexp != bRegexp {
		log.Debugf("Regexp mismatch: %v != %v", aRegexp, bRegexp)
		return false
	}

	log.Debugf("Endpoints match successfully.")
	return true
}

func (p *MikrotikProvider) listContains(haystack []*endpoint.Endpoint, needle *endpoint.Endpoint) bool {
	for _, v := range haystack {
		if p.compareEndpoints(needle, v) {
			return true
		}
	}
	return false
}

// changes processes and filters the changes plan for updates.
// It adjusts TTL for created endpoints and removes duplicate updates from the plan.
func (p *MikrotikProvider) changes(changes *plan.Changes) *plan.Changes {
	log.Debug("Starting to process changes plan.")

	// Initialize new plan -> we don't really need to worry about Delete changes.
	// Only updates are sketchy
	newChanges := &plan.Changes{
		Create:    []*endpoint.Endpoint{},
		Delete:    changes.Delete,
		UpdateOld: []*endpoint.Endpoint{},
		UpdateNew: []*endpoint.Endpoint{},
	}

	log.Debugf("Initial changes - Create: %d, Delete: %d, UpdateOld: %d, UpdateNew: %d", len(changes.Create), len(changes.Delete), len(changes.UpdateOld), len(changes.UpdateNew))

	// Process Create changes
	for _, create := range changes.Create {
		if !create.RecordTTL.IsConfigured() {
			log.Debugf("Setting default TTL for created endpoint: %v", create)
			create.RecordTTL = endpoint.TTL(p.client.TTL)
		}
		newChanges.Create = append(newChanges.Create, create)
	}

	// Identify duplicates in Update changes
	duplicates := []*endpoint.Endpoint{}
	for _, old := range changes.UpdateOld {
		for _, new := range changes.UpdateNew {
			if p.compareEndpoints(old, new) {
				log.Debugf("Found duplicate update for endpoint: %v", old)
				duplicates = append(duplicates, old)
			}
		}
	}

	// Filter out duplicates from UpdateOld
	for _, old := range changes.UpdateOld {
		if !p.listContains(duplicates, old) {
			log.Debugf("Adding non-duplicate UpdateOld endpoint: %v", old)
			newChanges.UpdateOld = append(newChanges.UpdateOld, old)
		}
	}

	// Filter out duplicates from UpdateNew
	for _, new := range changes.UpdateNew {
		if !p.listContains(duplicates, new) {
			log.Debugf("Adding non-duplicate UpdateNew endpoint: %v", new)
			if !new.RecordTTL.IsConfigured() {
				log.Debugf("Setting default TTL for UpdateNew endpoint: %v", new)
				new.RecordTTL = endpoint.TTL(p.client.TTL)
			}
			newChanges.UpdateNew = append(newChanges.UpdateNew, new)
		}
	}

	log.Debugf("Processed changes - Create: %d, Delete: %d, UpdateOld: %d, UpdateNew: %d", len(newChanges.Create), len(newChanges.Delete), len(newChanges.UpdateOld), len(newChanges.UpdateNew))
	log.Debug("Finished processing changes plan.")
	return newChanges
}
