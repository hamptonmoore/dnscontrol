package packetframe

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/StackExchange/dnscontrol/v3/models"
	"github.com/StackExchange/dnscontrol/v3/pkg/diff"
	"github.com/StackExchange/dnscontrol/v3/providers"
	"github.com/miekg/dns/dnsutil"
)

// packetframeProvider is the handle for this provider.
type packetframeProvider struct {
	client      *http.Client
	baseURL     *url.URL
	domainIndex map[string]domain
}

var defaultNameServerNames = []string{
	"ns1.packetframe.com",
	"ns2.packetframe.com",
}

// NewPacketframe creates the provider.
func NewPacketframe(m map[string]string, metadata json.RawMessage) (providers.DNSServiceProvider, error) {
	if m["apikey"] == "" {
		return nil, fmt.Errorf("missing Packetframe token")
	}

	cookie := &http.Cookie{
		Name:  "apikey",
		Value: m["apikey"],
	}
	cookies := make([]*http.Cookie, 1)
	cookies[0] = cookie
	urlObj, _ := url.Parse("https://packetframe.com/")
	jar, _ := cookiejar.New(nil)
	jar.SetCookies(urlObj, cookies)

	baseURL, err := url.Parse(defaultBaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL for Packetframe")
	}
	client := http.Client{Jar: jar}

	api := &packetframeProvider{client: &client, baseURL: baseURL}

	// Get a domain to validate the token
	if err := api.fetchDomainList(); err != nil {
		return nil, err
	}

	return api, nil
}

var features = providers.DocumentationNotes{
	providers.DocDualHost:            providers.Cannot(),
	providers.DocOfficiallySupported: providers.Cannot(),
	providers.CanGetZones:            providers.Unimplemented(),
}

func init() {
	fns := providers.DspFuncs{
		Initializer:   NewPacketframe,
		RecordAuditor: AuditRecords,
	}
	providers.RegisterDomainServiceProviderType("PACKETFRAME", fns, features)
}

// GetNameservers returns the nameservers for a domain.
func (api *packetframeProvider) GetNameservers(domain string) ([]*models.Nameserver, error) {
	return models.ToNameservers(defaultNameServerNames)
}

// GetZoneRecords gets the records of a zone and returns them in RecordConfig format.
func (api *packetframeProvider) GetZoneRecords(domain string) (models.Records, error) {
	return nil, fmt.Errorf("not implemented")
	// This enables the get-zones subcommand.
	// Implement this by extracting the code from GetDomainCorrections into
	// a single function.  For most providers this should be relatively easy.
}

// GetDomainCorrections returns the corrections for a domain.
func (api *packetframeProvider) GetDomainCorrections(dc *models.DomainConfig) ([]*models.Correction, error) {
	dc, err := dc.Copy()
	if err != nil {
		return nil, err
	}

	dc.Punycode()

	if api.domainIndex == nil {
		if err := api.fetchDomainList(); err != nil {
			return nil, err
		}
	}
	_, ok := api.domainIndex[dc.Name]
	if !ok {
		return nil, fmt.Errorf("'%s' not a zone in Packetframe account", dc.Name)
	}

	records := api.domainIndex[dc.Name].Records

	existingRecords := make([]*models.RecordConfig, len(records))

	for i := range records {
		existingRecords[i] = toRc(dc, &records[i])
	}

	// Normalize
	// models.PostProcessRecords(existingRecords)

	for _, record := range dc.Records {
		if record.Name == "@" {
			record.Name = dc.Name
			record.NameFQDN = dc.Name + "."
		}
	}

	differ := diff.New(dc)
	_, create, _, _, err := differ.IncrementalDiff(existingRecords)
	if err != nil {
		return nil, err
	}

	var corrections []*models.Correction

	for _, m := range create {
		// log.Println(m.Desired.String())
		req, err := toReq(dc, m.Desired)
		j, err := json.Marshal(req)
		// log.Println(j)
		if err != nil {
			return nil, err
		}
		corr := &models.Correction{
			Msg: fmt.Sprintf("%s: %s", m.String(), string(j)),
			F: func() error {
				_, err := api.createRecord(dc.Name, req)
				return err
			},
		}
		corrections = append(corrections, corr)
	}

	return corrections, nil
}

func toReq(dc *models.DomainConfig, rc *models.RecordConfig) (*domainRecord, error) {
	req := &domainRecord{
		Type:  rc.Type,
		TTL:   int(rc.TTL),
		Label: rc.GetLabelFQDN(),
	}

	switch rc.Type { // #rtype_variations
	case "A", "AAAA", "PTR", "TXT", "CNAME", "MX":
		req.Value = rc.GetTargetField()
	default:
		return nil, fmt.Errorf("packetframe.toReq rtype %q unimplemented", rc.Type)
	}

	return req, nil
}

func toRc(dc *models.DomainConfig, r *domainRecord) *models.RecordConfig {
	rc := &models.RecordConfig{
		Type:     r.Type,
		TTL:      uint32(r.TTL),
		Original: r,
	}
	rc.SetLabel(r.Label, dc.Name)

	switch rtype := r.Type; rtype { // #rtype_variations
	case "TXT":
		rc.SetTargetTXT(r.Value)
	case "CNAME", "MX", "NS", "SRV":
		rc.SetTarget(dnsutil.AddOrigin(r.Value+".", dc.Name))
	default:
		rc.SetTarget(r.Value)
	}

	return rc
}
