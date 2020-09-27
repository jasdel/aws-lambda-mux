package lambdamux

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

// APIGatewayProxy provides an Lambda Handler for proxied Lambda invokes from
// API Gateway.
type APIGatewayProxy struct {
	Handler ResourceHandler
}

// APIGatewayProxyRequest provides a proxy request wrapper for deserializing
// the events.APIGatewayProxyRequest with Go's http.Header formated headers.
// Simplifies the conversion between Go's http.Header and lambda's events multi
// value header parameter.
type APIGatewayProxyRequest struct {
	events.APIGatewayProxyRequest
	HTTPHeader http.Header `json:"-"`
}

// UnmarshalJSON unmarshals APIGatewayProxyRequest with the MultiValueHeaders
// deserialized as Go http.Header.
func (r *APIGatewayProxyRequest) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &r.APIGatewayProxyRequest); err != nil {
		return err
	}

	r.HTTPHeader = http.Header{}
	for header, values := range r.MultiValueHeaders {
		for _, value := range values {
			r.HTTPHeader.Add(header, value)
		}
	}

	return nil
}

// APIGatewayProxyResponse serializes the events.APIGatewayResponse with Go's
// http.Header serialized as a MultiValueHeaders. Simplifies the conversion
// between Go's http.Header and lambda's events multi value header parameter.
type APIGatewayProxyResponse struct {
	events.APIGatewayProxyResponse
	HTTPHeader http.Header `json:"-"`
}

// MarshalJSON marshals the response as an JSON document.
func (r APIGatewayProxyResponse) MarshalJSON() ([]byte, error) {
	r.MultiValueHeaders = map[string][]string(r.HTTPHeader)

	return json.Marshal(r.APIGatewayProxyResponse)
}

// Invoke invokes the API Gateway API call. Implements lambda's Handler
// interface.
//
// Deserializes the request as an APIGatewayProxyRequest, and serializes the
// response as a APIGatewayProxyResponse.
func (p APIGatewayProxy) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	var req APIGatewayProxyRequest

	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("invalid lambda event, expect %T, %w", req, err)
	}

	resp, err := p.Handler.ServeResource(ctx, req)
	if err != nil {
		return nil, err
	}

	out, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %T, %w", resp, err)
	}

	return out, nil
}

// ResourceHandler is the interface for APIGatewayProxy Lambda resource
// handlers for implementations to provide resource handlers.
type ResourceHandler interface {
	ServeResource(context.Context, APIGatewayProxyRequest) (APIGatewayProxyResponse, error)
}

// ServeResource is an API Gateway Proxy Lambda resource handler that matches
// API Gateway request resources by exact name. Delegates to the resource
// handler by name.
//
// Resource name must match exactly, including path parameters.
type ServeResource struct {
	resources map[string]ResourceHandler
}

// NewServeResource initializes and returns a ServeResource that resource
// handlers can be added to via the Handle method.
func NewServeResource() *ServeResource {
	return &ServeResource{resources: map[string]ResourceHandler{}}
}

// ServeResource implements the ResourceHandler interface, and delegates the
// requests to the registered handler. If no handler is found returns an error.
func (s *ServeResource) ServeResource(
	ctx context.Context, req APIGatewayProxyRequest,
) (resp APIGatewayProxyResponse, err error) {
	h, ok := s.resources[req.Resource]
	if !ok {
		return resp, fmt.Errorf("resource handler not found for %s", req.Resource)
	}
	return h.ServeResource(ctx, req)
}

// Handle adds a new resource handler for the resource.
func (s *ServeResource) Handle(resource string, handler ResourceHandler) *ServeResource {
	s.resources[resource] = handler
	return s
}

// ServeMethod is an API Gateway Proxy resource handler delegating resource
// requests to resource handlers filtered by HTTP request method.
type ServeMethod struct {
	methods map[string]ResourceHandler
}

// NewServeMethod initializes and returns a ServeMethod that HTTP methods can
// be added to via the Handle method.
func NewServeMethod() *ServeMethod {
	return &ServeMethod{methods: map[string]ResourceHandler{}}
}

// ServeResource implements the ResourceHandler interface, delegating resource
// requests to the ResourceHandler associated with the HTTP request method.
func (s *ServeMethod) ServeResource(
	ctx context.Context, req APIGatewayProxyRequest,
) (resp APIGatewayProxyResponse, err error) {
	h, ok := s.methods[req.HTTPMethod]
	if !ok {
		return resp, fmt.Errorf("method handler not found for %s:%s", req.Resource, req.HTTPMethod)
	}
	return h.ServeResource(ctx, req)
}

// Handle adds a new ResourceHandler associated with a HTTP request method.
// Replaces existing methods that match.
//
// HTTP request methods are not case sensitive.
func (s *ServeMethod) Handle(method string, handler ResourceHandler) *ServeMethod {
	s.methods[strings.ToUpper(method)] = handler

	return s
}

// ResourceHandlerFunc provides wrapping of a function as the ResourceHandler.
type ResourceHandlerFunc func(context.Context, APIGatewayProxyRequest) (
	resp APIGatewayProxyResponse, err error,
)

// ServeResource implements the ResourceHandler interface and delegates to the
// function to handle the resource.
func (f ResourceHandlerFunc) ServeResource(
	ctx context.Context, req APIGatewayProxyRequest,
) (resp APIGatewayProxyResponse, err error) {
	return f(ctx, req)
}
