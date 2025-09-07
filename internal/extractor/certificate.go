package extractor

import (
	"fmt"
	"net/http"
	"time"
)

// CertificateInfo contains SSL/TLS certificate information extracted from HTTPS responses.
// All fields represent commonly accessed certificate attributes for validation and debugging.
type CertificateInfo struct {
	Subject      string    `json:"subject"`
	Issuer       string    `json:"issuer"`
	ExpireDate   time.Time `json:"expire_date"`
	SerialNumber string    `json:"serial_number"`
}

// ExtractAllCertificateFields extracts comprehensive SSL/TLS certificate information.
// Returns structured certificate data from the first peer certificate in the TLS connection.
// Useful for certificate validation, debugging, and compliance checking.
// Returns ErrInvalidInput if response is nil.
// Returns ErrNotFound if no TLS connection exists or no peer certificates are available.
func ExtractAllCertificateFields(resp *http.Response) (*CertificateInfo, error) {
	if resp == nil {
		return nil, fmt.Errorf("%w: response is nil", ErrInvalidInput)
	}

	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		return nil, ErrNotFound
	}

	cert := resp.TLS.PeerCertificates[0]

	return &CertificateInfo{
		Subject:      cert.Subject.String(),
		Issuer:       cert.Issuer.String(),
		ExpireDate:   cert.NotAfter,
		SerialNumber: cert.SerialNumber.String(),
	}, nil
}
