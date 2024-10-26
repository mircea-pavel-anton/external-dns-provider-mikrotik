package mikrotik

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/external-dns/endpoint"
)

// DNSRecord represents a MikroTik DNS record in the format used by the API
// https://help.mikrotik.com/docs/display/ROS/DNS#DNS-DNSStatic
type DNSRecord struct {
	// Common fields for all record types
	ID             string `json:".id,omitempty"`             // only fetched from API
	Name           string `json:"name"`                      // endpoint.DNSName
	Type           string `json:"type"`                      // endpoint.RecordType
	TTL            string `json:"ttl,omitempty"`             // endpoint.RecordTTL
	Comment        string `json:"comment,omitempty"`         // provider-specific
	Regexp         string `json:"regexp,omitempty"`          // provider-specific
	MatchSubdomain string `json:"match-subdomain,omitempty"` // provider-specific
	AddressList    string `json:"address-list,omitempty"`    // provider-specific

	// Disabled       string `json:"disabled,omitempty"`        // provider-specific

	// Record specific fields
	Address string `json:"address,omitempty"` // A, AAAA -> endpoint.Targets[0]
	CName   string `json:"cname,omitempty"`   // CNAME -> endpoint.Targets[0]
	Text    string `json:"text,omitempty"`    // TXT -> endpoint.Targets[0]

	MXExchange   string `json:"mx-exchange,omitempty"`   // MX -> provider-specific
	MXPreference string `json:"mx-preference,omitempty"` // MX -> provider-specific
	// Additional fields for other record types that are not currently supported
	// SrvPort      string `json:"srv-port,omitempty"`      // SRV -> provider-specific
	// SrvTarget    string `json:"srv-target,omitempty"`    // SRV -> provider-specific
	// SrvPriority  string `json:"srv-priority,omitempty"`  // SRV -> provider-specific
	// SrvWeight    string `json:"srv-weight,omitempty"`    // SRV -> provider-specific
	// NS           string `json:"ns,omitempty"`            // NS -> provider-specific
	// ForwardTo    string `json:"forward-to,omitempty"`    // FWD
}

// NewDNSRecord converts an ExternalDNS Endpoint to a Mikrotik DNSRecord
func NewDNSRecord(endpoint *endpoint.Endpoint) (*DNSRecord, error) {
	log.Debugf("Converting ExternalDNS endpoint to MikrotikDNS: %v", endpoint)

	// Sanity checks -> Fields are not empty and if set, they are set correctly
	if endpoint.DNSName == "" {
		return nil, fmt.Errorf("DNS name is required")
	}
	if endpoint.RecordType == "" {
		return nil, fmt.Errorf("record type is required")
	}
	if len(endpoint.Targets) == 0 || endpoint.Targets[0] == "" {
		return nil, fmt.Errorf("no target provided for DNS record")
	}

	// Convert ExternalDNS TTL to Mikrotik TTL
	ttl, err := endpointTTLtoMikrotikTTL(endpoint.RecordTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to convert TTL: %v", err)
	}

	// Initialize new records
	record := &DNSRecord{Name: endpoint.DNSName, Type: endpoint.RecordType, TTL: ttl}
	log.Debugf("Name set to: %s", record.Name)
	log.Debugf("Type set to: %s", record.Type)
	log.Debugf("TTL set to: %s", record.TTL)

	// Record-type specific data
	switch record.Type {
	case "A":
		if err := validateIPv4(endpoint.Targets[0]); err != nil {
			return nil, err
		}
		record.Address = endpoint.Targets[0]
		log.Debugf("Address set to: %s", record.Address)

	case "AAAA":
		if err := validateIPv6(endpoint.Targets[0]); err != nil {
			return nil, err
		}
		record.Address = endpoint.Targets[0]
		log.Debugf("Address set to: %s", record.Address)

	case "CNAME":
		if err := validateDomain(endpoint.Targets[0]); err != nil {
			return nil, err
		}
		record.CName = endpoint.Targets[0]
		log.Debugf("CNAME set to: %s", record.Address)

	case "TXT":
		if err := validateTXT(endpoint.Targets[0]); err != nil {
			return nil, err
		}
		record.Text = endpoint.Targets[0]
		log.Debugf("Text set to: %s", record.Text)

	case "MX":
		preference, exchange, err := parseMX(endpoint.Targets[0])
		if err != nil {
			return nil, err
		}
		record.MXPreference = fmt.Sprintf("%v", preference)
		log.Debugf("MX preference set to: %s", record.MXPreference)
		record.MXExchange = exchange
		log.Debugf("MX exchange set to: %s", record.MXExchange)

	default:
		return nil, fmt.Errorf("unsupported DNS type: %s", endpoint.RecordType)
	}

	for _, providerSpecific := range endpoint.ProviderSpecific {
		switch providerSpecific.Name {
		case "comment":
			record.Comment = providerSpecific.Value
			log.Debugf("Comment set to: %s", record.Comment)
		case "regexp":
			record.Regexp = providerSpecific.Value
			log.Debugf("Regexp set to: %s", record.Regexp)
		case "match-subdomain":
			record.MatchSubdomain = providerSpecific.Value
			log.Debugf("MatchSubdomain set to: %s", record.MatchSubdomain)
		case "address-list":
			record.AddressList = providerSpecific.Value
			log.Debugf("AddressList set to: %s", record.AddressList)
		default:
			return nil, fmt.Errorf(
				"unsupported provider specific configuration '%s' for DNS Record of type %s",
				providerSpecific.Name,
				record.Type,
			)
		}
	}

	log.Debugf("Converted ExternalDNS endpoint to MikrotikDNS: %v", record)
	return record, nil
}

// toExternalDNSEndpoint converts a Mikrotik DNSRecord to an ExternalDNS Endpoint
func (r *DNSRecord) toExternalDNSEndpoint() (*endpoint.Endpoint, error) {
	log.Debugf("Converting MikrotikDNS record to ExternalDNS: %v", r)

	// ============================================================================================
	// Sanity checks
	// ============================================================================================
	if r.Name == "" {
		return nil, fmt.Errorf("DNS record name cannot be empty")
	}

	//? Mikrotik assumes A-records are default and sometimes omits setting the type
	if r.Type == "" {
		log.Debugf("Record type not set. Using default value 'A'")
		r.Type = "A"
	}

	ttl, err := mikrotikTTLtoEndpointTTL(r.TTL)
	if err != nil {
		return nil, fmt.Errorf("failed to convert MikrotikDNS record to ExternalDNS: %v", err)
	}

	// Initialize endpoint
	ep := endpoint.Endpoint{
		DNSName:    r.Name,
		RecordType: r.Type,
		RecordTTL:  ttl,
	}

	// ============================================================================================
	// Record-specific data
	// ============================================================================================
	switch ep.RecordType {
	case "A":
		if err := validateIPv4(r.Address); err != nil {
			return nil, err
		}
		ep.Targets = endpoint.NewTargets(r.Address)
		log.Debugf("Address set to: %s", r.Address)

	case "AAAA":
		if err := validateIPv6(r.Address); err != nil {
			return nil, err
		}
		ep.Targets = endpoint.NewTargets(r.Address)
		log.Debugf("Address set to: %s", r.Address)

	case "CNAME":
		if err := validateDomain(r.CName); err != nil {
			return nil, err
		}
		ep.Targets = endpoint.NewTargets(r.CName)
		log.Debugf("CNAME set to: %s", r.CName)

	case "TXT":
		if err := validateTXT(r.Text); err != nil {
			return nil, err
		}
		ep.Targets = endpoint.NewTargets(r.Text)
		log.Debugf("Text set to: %s", r.Text)

	case "MX":
		if err := validateDomain(r.MXExchange); err != nil {
			return nil, err
		}
		if err := validateMXPreference(r.MXPreference); err != nil {
			return nil, err
		}
		ep.Targets = endpoint.NewTargets(fmt.Sprintf("%s %s", r.MXPreference, r.MXExchange))
		log.Debugf("MX preference set to: %s", r.MXPreference)
		log.Debugf("MX exchange set to: %s", r.MXExchange)

	default:
		return nil, fmt.Errorf("unsupported DNS type: %s", ep.RecordType)
	}

	// Ensure at least one target is present and non-empty
	if len(ep.Targets) == 0 || ep.Targets[0] == "" {
		return nil, fmt.Errorf("no target provided for DNS record")
	}

	// ============================================================================================
	// Provider-specific stuff
	// ============================================================================================
	if r.Comment != "" {
		ep.ProviderSpecific = append(ep.ProviderSpecific, endpoint.ProviderSpecificProperty{
			Name:  "comment",
			Value: r.Comment,
		})
	}
	if r.Regexp != "" {
		ep.ProviderSpecific = append(ep.ProviderSpecific, endpoint.ProviderSpecificProperty{
			Name:  "regexp",
			Value: r.Regexp,
		})
	}
	if r.MatchSubdomain != "" {
		ep.ProviderSpecific = append(ep.ProviderSpecific, endpoint.ProviderSpecificProperty{
			Name:  "match-subdomain",
			Value: r.MatchSubdomain,
		})
	}
	if r.AddressList != "" {
		ep.ProviderSpecific = append(ep.ProviderSpecific, endpoint.ProviderSpecificProperty{
			Name:  "address-list",
			Value: r.AddressList,
		})
	}

	log.Debugf("Converted MikrotikDNS record to ExternalDNS: %v", ep)
	return &ep, nil
}

// ================================================================================================
// UTILS
// ================================================================================================
// mikrotikTTLtoEndpointTTL converts a Mikrotik TTL to an ExternalDNS TTL
func mikrotikTTLtoEndpointTTL(ttl string) (endpoint.TTL, error) {
	log.Debugf("Converting Mikrotik TTL to Endpoint TTL: %s", ttl)

	if ttl == "" {
		log.Warnf("Found a Mikrotik Endpoint with no TTL?! Setting TTL to 0")
		ttl = "0s"
	}

	// Define the unit multipliers in seconds
	unitMap := map[string]float64{
		"d": 86400, // seconds in a day
		"h": 3600,  // seconds in an hour
		"m": 60,    // seconds in a minute
		"s": 1,     // seconds in a second
	}

	// Regular expression to match number-unit pairs, including negative numbers
	re := regexp.MustCompile(`(-?\d*\.?\d+)([dhms])`)

	matches := re.FindAllStringSubmatch(ttl, -1)
	if matches == nil {
		return 0, fmt.Errorf("invalid duration string: '%s'", ttl)
	}

	// Reconstruct the matched parts to validate the entire input
	var matchedString string
	for _, match := range matches {
		matchedString += match[0]
	}

	// Remove any whitespace for accurate comparison
	trimmedInput := strings.ReplaceAll(ttl, " ", "")
	if matchedString != trimmedInput {
		return 0, fmt.Errorf("invalid characters in duration string: '%s'", ttl)
	}

	var totalSeconds float64

	for _, match := range matches {
		valueStr := match[1]
		unitStr := match[2]

		multiplier, ok := unitMap[unitStr]
		if !ok {
			return 0, fmt.Errorf("invalid unit '%s' in duration", unitStr)
		}

		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number '%s' in duration", valueStr)
		}

		if value < 0 {
			return 0, fmt.Errorf("negative values are not allowed: '%s'", valueStr+unitStr)
		}

		totalSeconds += value * multiplier
	}

	duration := time.Duration(totalSeconds * float64(time.Second))
	log.Debugf("Parsed duration: %v", duration)

	log.Debugf("Converted TTL: %v", duration.Seconds())
	return endpoint.TTL(duration.Seconds()), nil
}

// endpointTTLtoMikrotikTTL converts an ExternalDNS TTL to a Mikrotik TTL.
// If no TTL is configured in the ExternalDNS endpoint, the default TTL is used.
func endpointTTLtoMikrotikTTL(ttl endpoint.TTL) (string, error) {
	log.Debugf("Converting Endpoint TTL to Mikrotik TTL: %v", ttl)

	if ttl < 0 {
		return "", fmt.Errorf("negative TTL values are not allowed: %v", ttl)
	}

	totalSeconds := int64(ttl)
	days := totalSeconds / 86400
	remainder := totalSeconds % 86400

	hours := remainder / 3600
	remainder %= 3600

	minutes := remainder / 60
	seconds := remainder % 60

	var parts []string

	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}

	durationStr := strings.Join(parts, "")
	log.Debugf("Converted TTL: %v", durationStr)
	return durationStr, nil
}

// validateIPv4 checks if the provided address is a valid IPv4 address.
func validateIPv4(address string) error {
	if net.ParseIP(address) == nil {
		return fmt.Errorf("invalid IP address: %s", address)
	}

	if strings.Contains(address, ":") {
		return fmt.Errorf("provided address looks like an IPv6 address: %s", address)
	}

	return nil
}

// validateIPv6 checks if the provided address is a valid IPv6 address.
func validateIPv6(address string) error {
	if net.ParseIP(address) == nil {
		return fmt.Errorf("invalid IP address: %s", address)
	}

	if !strings.Contains(address, ":") {
		return fmt.Errorf("provided address looks like an IPv4 address: %s", address)
	}

	return nil
}

// validateTXT checks if the provided TXT record text is valid.
func validateTXT(text string) error {
	if text == "" {
		return fmt.Errorf("TXT record text cannot be empty")
	}
	//? TODO: add more validation here?
	return nil
}

// validateDomain checks if the provided domain is semantically valid.
func validateDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("a domain cannot be empty")
	}

	if len(domain) > 253 {
		return fmt.Errorf("invalid domain, length exceeds 253 characters")
	}

	domainRegex := `^(?i:[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?\.)+[a-z]{2,}$`
	matched, err := regexp.MatchString(domainRegex, domain)
	if err != nil || !matched {
		return fmt.Errorf("invalid domain: %s", domain)
	}

	return nil
}

// validateMXPreference checks if the provided MX preference is valid.
func validateMXPreference(preference string) error {
	if preference == "" {
		return fmt.Errorf("MX preference cannot be empty")
	}
	preferenceVal, err := strconv.Atoi(preference)
	if err != nil {
		return fmt.Errorf("invalid MX preference value: %s . Value cannot be converrted to int", preference)
	}

	if preferenceVal < 0 || preferenceVal > 65535 {
		return fmt.Errorf("invalid MX preference value: %s . Value must be between 0 and 65535", preference)
	}
	return nil
}

// parseMX parses and validates an MX record
func parseMX(data string) (string, string, error) {
	data_split := strings.Split(data, " ")
	if len(data_split) != 2 {
		return "", "", fmt.Errorf("malformed MX record %s", data)
	}

	// Extract and Validate MX Preference
	preference := data_split[0]
	if err := validateMXPreference(preference); err != nil {
		return "", "", err
	}

	// Extract and Validate MX Exchange
	exchange := data_split[1]
	if err := validateDomain(exchange); err != nil {
		return "", "", err
	}

	return preference, exchange, nil
}
