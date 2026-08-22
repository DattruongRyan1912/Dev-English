package ai

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/DattruongRyan1912/Dev-English/backend/internal/domain"
)

func startProbe(provider, capability, model string, configured bool) domain.ProviderCheck {
	check := domain.ProviderCheck{
		Provider:   provider,
		Configured: configured,
		Capability: capability,
		Model:      model,
		Status:     "not_configured",
	}
	if !configured {
		check.Error = "credentials_or_endpoint_missing"
	}
	return check
}

func doProbeRequest(check domain.ProviderCheck, client *http.Client, req *http.Request) (domain.ProviderCheck, *http.Response) {
	started := time.Now()
	resp, err := client.Do(req)
	check.LatencyMs = time.Since(started).Milliseconds()
	if err != nil {
		return finishProbeError(check, err), nil
	}
	check.Reachable = true
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		check.Status = "unhealthy"
		check.Error = safeHTTPProbeError(resp.StatusCode)
		resp.Body.Close()
		return check, nil
	}
	check.Healthy = true
	check.Status = "healthy"
	return check, resp
}

func finishProbeError(check domain.ProviderCheck, err error) domain.ProviderCheck {
	check.Healthy = false
	check.Status = "unhealthy"
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		check.Error = "timeout"
	case errors.Is(err, context.Canceled):
		check.Error = "canceled"
	case errors.As(err, new(net.Error)):
		check.Error = "provider_unreachable"
	default:
		check.Error = "invalid_provider_response"
	}
	return check
}

func safeHTTPProbeError(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "authentication_failed"
	case http.StatusNotFound:
		return "endpoint_not_found"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusBadRequest:
		return "provider_rejected_probe"
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return "provider_server_error"
	default:
		return "provider_http_error"
	}
}
