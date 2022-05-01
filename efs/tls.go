package efs

import "crypto/tls"

// LoadX509KeyPair reads and parses a public/private key pair from a pair
// of files. The files must contain PEM encoded data. The certificate file
// may contain intermediate certificates following the leaf certificate to
// form a certificate chain. On successful return, Certificate.Leaf will
// be nil because the parsed form of the certificate is not retained.
func LoadX509KeyPair(certFile, keyFile string) (tls.Certificate, error) {
	certPEMBlock, err := ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEMBlock, err := ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(certPEMBlock, keyPEMBlock)
}
