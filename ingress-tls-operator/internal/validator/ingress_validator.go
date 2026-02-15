package validator

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type IngressValidator struct {
	client *http.Client

	trustAnchors []byte
	tlsInsecure  bool
	timeout      time.Duration
	proxyAddress string
}

type validationResponse struct {
	Status     string // e.g. "200 OK"
	StatusCode int    // e.g. 200
	Message    string
	Error      error
}

func (iv *IngressValidator) generateURL(hostname string, path string) string {
	targetUrl := &url.URL{
		Scheme: "http",
		Host:   hostname,
		Path:   path,
	}
	return targetUrl.String()
}

func (iv *IngressValidator) parseEndpoints(ingress *networkingv1.Ingress) []string {
	endpoints := make([]string, 0)
	for _, rule := range ingress.Spec.Rules {
		if len(rule.Host) == 0 {
			continue
		}
		hostname := rule.Host

		if rule.HTTP == nil {
			endpoints = append(endpoints, iv.generateURL(hostname, "/"))
			continue
		}

		for _, path := range rule.HTTP.Paths {
			endpoints = append(endpoints, iv.generateURL(hostname, path.Path))
		}
	}
	return endpoints
}

type EndPoint struct {
	Path    string
	Code    int
	Message string
}

type Response struct {
	OK bool

	IngressName string
	Namespace   string

	Endpoints []*EndPoint
}

var (
	ErrInsecureEndpoint = errors.New("insecure endpoint detected")
	ErrInternalError    = errors.New("internal error")
)

func (r *Response) Log(ctx context.Context) {
	logger := log.FromContext(ctx)
	for _, endpoint := range r.Endpoints {
		switch {
		case endpoint.Code >= http.StatusMultipleChoices &&
			endpoint.Code <= http.StatusIMUsed:

		case endpoint.Code >= http.StatusOK &&
			endpoint.Code <= http.StatusIMUsed:
			logger.Info("CRITICAL: no TLS configured",
				"namespace", r.Namespace,
				"name", r.IngressName,
				"path", endpoint.Path,
				"level", "critical")
		default:
			logger.Info("WARNING: internal error",
				"namespace", r.Namespace,
				"name", r.IngressName,
				"path", endpoint.Path,
				"level", "warning")
		}
	}
}

func (iv *IngressValidator) Inspect(ctx context.Context, ingress *networkingv1.Ingress) *Response {
	addresses := iv.parseEndpoints(ingress)
	if len(addresses) == 0 {
		return &Response{
			OK: true,
		}
	}

	endpoints := make([]*EndPoint, 0)
	for _, targetAddress := range addresses {
		validationresponse := iv.Do(ctx, targetAddress)
		if validationresponse.Error != nil {
			endpoints = append(endpoints, &EndPoint{
				Path: targetAddress,
				Code: validationresponse.StatusCode,
				Message: fmt.Sprintf("internal error:%s",
					validationresponse.Error),
			})
			continue
		}
		switch {
		case validationresponse.StatusCode >= http.StatusMultipleChoices &&
			validationresponse.StatusCode <= http.StatusPermanentRedirect:
			// redirect configured, skip
		case validationresponse.StatusCode >= http.StatusOK &&
			validationresponse.StatusCode <= http.StatusIMUsed:
			endpoints = append(endpoints, &EndPoint{
				Path:    targetAddress,
				Code:    validationresponse.StatusCode,
				Message: "insecure: no tls configured",
			})
		default:
			endpoints = append(endpoints, &EndPoint{
				Path: targetAddress,
				Code: validationresponse.StatusCode,
				Message: fmt.Sprintf("internal error:%s",
					validationresponse.Error),
			})
		}
	}

	return &Response{
		Namespace:   ingress.Namespace,
		IngressName: ingress.Name,
		Endpoints:   endpoints,
		OK:          len(endpoints) == 0,
	}
}

func (iv *IngressValidator) Do(ctx context.Context, targetAddress string) *validationResponse {
	select {
	case <-ctx.Done():
		return &validationResponse{
			Error: fmt.Errorf("context expired"),
		}
	default:
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetAddress, nil)
	if err != nil {
		return &validationResponse{
			Error:   err,
			Message: "invalid request",
		}
	}

	res, err := iv.client.Do(req)
	if err != nil {
		return &validationResponse{
			Error:   err,
			Message: "unreachable url",
		}
	}
	defer func() {
		_ = res.Body.Close()
	}()
	defer func() {
		_, _ = io.Copy(io.Discard, res.Body)
	}()

	return &validationResponse{
		Status:     res.Status,
		StatusCode: res.StatusCode,
	}
}

func WithTrustAnchors(trustAnchors []byte) func(*IngressValidator) {
	return func(iv *IngressValidator) {
		iv.trustAnchors = trustAnchors
	}
}

func WithTLSInsecureVerify(tlsInsecure bool) func(*IngressValidator) {
	return func(iv *IngressValidator) {
		iv.tlsInsecure = tlsInsecure
	}
}

func WithTimeout(duration time.Duration) func(*IngressValidator) {
	return func(iv *IngressValidator) {
		iv.timeout = duration
	}
}

func WithProxy(proxyAddress string) func(*IngressValidator) {
	return func(iv *IngressValidator) {
		iv.proxyAddress = proxyAddress
	}
}

func NewIngressValidator(ctx context.Context, args ...func(*IngressValidator)) (*IngressValidator, error) {
	logger := log.FromContext(ctx)
	ingressValidator := new(IngressValidator)
	for _, f := range args {
		f(ingressValidator)
	}

	var proxyUrl *url.URL
	var err error
	if len(strings.TrimSpace(ingressValidator.proxyAddress)) > 0 {
		proxyUrl, err = url.Parse(ingressValidator.proxyAddress)
		if err != nil {
			return ingressValidator, err
		}
	}

	trustedCAs, err := x509.SystemCertPool()
	if err != nil {
		return ingressValidator, err
	}
	if ok := trustedCAs.AppendCertsFromPEM(ingressValidator.trustAnchors); !ok {
		logger.Info("custom trust anchors, ignored", "trustanchors",
			base64.StdEncoding.EncodeToString(ingressValidator.trustAnchors))
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: ingressValidator.tlsInsecure,
		RootCAs:            trustedCAs,
	}

	transPort := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	if proxyUrl != nil {
		transPort.Proxy = http.ProxyURL(proxyUrl)
	}

	return &IngressValidator{
		client: &http.Client{
			Timeout:   ingressValidator.timeout,
			Transport: transPort,
			CheckRedirect: func(req *http.Request, via []*http.Request) error { // disable redirects
				return http.ErrUseLastResponse
			},
		},
	}, nil
}
