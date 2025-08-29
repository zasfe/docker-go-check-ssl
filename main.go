package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

// CertInfo는 개별 인증서의 상세 정보를 담는 구조체입니다.
type CertInfo struct {
	Subject       string    `json:"subject"`
	Issuer        string    `json:"issuer"`
	NotBefore     time.Time `json:"not_before"`
	NotAfter      time.Time `json:"not_after"`
	DNSNames      []string  `json:"dns_names,omitempty"`
	IsCA          bool      `json:"is_ca"`
	SignatureAlgo string    `json:"signature_algorithm"`
}

// Response는 API 응답의 전체 구조입니다.
type Response struct {
	TargetURL       string     `json:"target_url"`
	Certificates    []CertInfo `json:"certificates"`
	ChainValidation string     `json:"chain_validation_message"`
}

// SSL 인증서 확인 로직을 처리하는 핸들러
func checkSSLHandler(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	hostname := r.URL.Query().Get("url")

	if ip == "" || hostname == "" {
		http.Error(w, `{"error": "Query parameters 'ip' and 'url' are required."}`, http.StatusBadRequest)
		return
	}

	// URL에서 호스트명만 정확히 추출
	if u, err := url.Parse(hostname); err == nil && u.Host != "" {
		hostname = u.Host
	} else if u, err := url.Parse("https://" + hostname); err == nil && u.Host != "" {
		hostname = u.Host
	}

	address := net.JoinHostPort(ip, "443")

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{
		ServerName:         hostname,
		InsecureSkipVerify: true,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to connect via TLS: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		http.Error(w, `{"error": "Server did not provide any certificates."}`, http.StatusInternalServerError)
		return
	}

	var certInfos []CertInfo
	for i, cert := range certs {
		info := CertInfo{
			Subject:       cert.Subject.String(),
			Issuer:        cert.Issuer.String(),
			NotBefore:     cert.NotBefore.UTC(),
			NotAfter:      cert.NotAfter.UTC(),
			IsCA:          cert.IsCA,
			SignatureAlgo: cert.SignatureAlgorithm.String(),
		}
		if i == 0 {
			info.DNSNames = cert.DNSNames
		}
		certInfos = append(certInfos, info)
	}

	intermediates := x509.NewCertPool()
	for i, cert := range certs {
		if i > 0 {
			intermediates.AddCert(cert)
		}
	}

	validationOpts := x509.VerifyOptions{
		DNSName:       hostname,
		Intermediates: intermediates,
	}

	validationMessage := "Certificate chain is valid."
	if _, err := certs[0].Verify(validationOpts); err != nil {
		validationMessage = fmt.Sprintf("Certificate chain verification failed: %s", err.Error())
	}

	responsePayload := Response{
		TargetURL:       fmt.Sprintf("https://%s", hostname),
		Certificates:    certInfos,
		ChainValidation: validationMessage,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responsePayload)
}

func main() {
	http.HandleFunc("/check-ssl", checkSSLHandler)
	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("could not start server: %s\n", err.Error())
	}
}

