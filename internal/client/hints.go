package client

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
)

// ErrorHint inspects an error and returns an actionable suggestion.
// Returns an empty string if no specific hint applies.
func ErrorHint(err error) string {
	if err == nil {
		return ""
	}

	var connErr *ConnectionError
	if errors.As(err, &connErr) {
		return connectionHint(connErr.Err)
	}

	var esErr *ESError
	if errors.As(err, &esErr) {
		return esErrorHint(esErr)
	}

	return connectionHint(err)
}

func connectionHint(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return fmt.Sprintf("Could not resolve hostname %q — check your --host URL.", dnsErr.Name)
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Op == "dial" {
			if errors.Is(opErr.Err, syscall.ECONNREFUSED) || strings.Contains(msg, "connection refused") {
				return "Connection refused — is Elasticsearch running? Check the --host URL and port."
			}
		}
	}

	if errors.Is(err, syscall.ECONNREFUSED) || strings.Contains(msg, "connection refused") {
		return "Connection refused — is Elasticsearch running? Check the --host URL and port."
	}

	if errors.Is(err, syscall.ECONNRESET) || strings.Contains(msg, "connection reset") {
		return "Connection was reset by the server — check if a proxy or firewall is in the way."
	}

	var certErr *x509.UnknownAuthorityError
	if errors.As(err, &certErr) {
		return "TLS certificate is not trusted — use --insecure to skip verification, or install the CA certificate."
	}

	var hostErr *x509.HostnameError
	if errors.As(err, &hostErr) {
		return "TLS certificate hostname mismatch — use --insecure to skip verification."
	}

	var certInvalidErr *x509.CertificateInvalidError
	if errors.As(err, &certInvalidErr) {
		return "TLS certificate is invalid — use --insecure to skip verification."
	}

	if strings.Contains(msg, "tls:") || strings.Contains(msg, "x509:") || strings.Contains(msg, "certificate") {
		return "TLS/certificate error — try --insecure if using self-signed certificates."
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Timeout() {
		return "Request timed out — try --timeout with a longer duration, or check cluster health."
	}

	if errors.Is(err, os.ErrDeadlineExceeded) || strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "context deadline") {
		return "Request timed out — try --timeout with a longer duration, or check cluster health."
	}

	if strings.Contains(msg, "no such host") {
		return "Hostname not found — check your --host URL."
	}

	if strings.Contains(msg, "connection refused") {
		return "Connection refused — is Elasticsearch running? Check the --host URL and port."
	}

	return ""
}

func esErrorHint(err *ESError) string {
	switch err.Status {
	case http.StatusUnauthorized:
		return "Authentication failed (401) — check your --user/--password or --api-key credentials."
	case http.StatusForbidden:
		return "Insufficient privileges (403) — your credentials lack the required permissions for this operation."
	case http.StatusNotFound:
		if strings.Contains(err.Reason, "no such index") || strings.Contains(err.Raw, "no such index") {
			return "Index not found — check the index name (run 'esql data' to list available indices)."
		}
		return "Resource not found (404) — check the index or endpoint name."
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return "Request timed out on the server — try a simpler query, add a LIMIT, or increase --timeout."
	case http.StatusBadRequest:
		if strings.Contains(err.Type, "parsing_exception") || strings.Contains(err.Type, "verification_exception") {
			return "Query syntax error — check your ES|QL query. Run 'esql' interactively for tab completion."
		}
	case http.StatusServiceUnavailable:
		return "Elasticsearch is unavailable (503) — the cluster may be starting up or under heavy load."
	case http.StatusTooManyRequests:
		return "Too many requests (429) — the cluster is overloaded. Wait a moment and try again."
	}
	return ""
}
