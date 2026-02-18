package extractor

import (
	"fmt"
	"net/http"
	"time"
)

const (
	CertificateFieldSubject      = "subject"
	CertificateFieldIssuer       = "issuer"
	CertificateFieldExpireDate   = "expire_date"
	CertificateFieldSerialNumber = "serial_number"
)

type CertificateInfo struct {
	Subject      string    `json:"subject"`
	Issuer       string    `json:"issuer"`
	ExpireDate   time.Time `json:"expire_date"`
	SerialNumber string    `json:"serial_number"`
}

// ExtractAllCertificateFields uses the first peer certificate in the TLS connection.
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

func ExtractCertificateField(resp *http.Response, field string) (any, error) {
	certInfo, err := ExtractAllCertificateFields(resp)
	if err != nil {
		return nil, err
	}

	switch field {
	case CertificateFieldSubject:
		return certInfo.Subject, nil
	case CertificateFieldIssuer:
		return certInfo.Issuer, nil
	case CertificateFieldExpireDate:
		return certInfo.ExpireDate.Format(time.RFC3339), nil
	case CertificateFieldSerialNumber:
		return certInfo.SerialNumber, nil
	default:
		return nil, fmt.Errorf("%w: unsupported certificate field: %s", ErrInvalidInput, field)
	}
}
