package types

import (
	"strings"
)

type Certificate string
type Certificates []Certificate
type PrivateKey string

func (certs Certificates) String() string {
	var sb strings.Builder
	for _, c := range certs {
		sb.WriteString(string(c))
	}
	return sb.String()
}

type CertificateAndKey struct {
	Certificate Certificate
	PrivateKey  PrivateKey
}
