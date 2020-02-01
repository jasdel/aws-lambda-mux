package lambdamux

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
)

// APIGatewayProxy provides an Lambda Handler for proxied Lambda invokes from
// API Gateway.
type APIGatewayProxy struct {
	Handler ResourceHandler
}

type APIGatewayProxyRequest struct {
	events.APIGatewayProxyRequest
	HTTPHeader http.Header `json:"-"`
}

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

type APIGatewayProxyResponse struct {
	events.APIGatewayProxyResponse
	HTTPHeader http.Header `json:"-"`
}

func (r APIGatewayProxyResponse) MarshalJSON() ([]byte, error) {
	r.MultiValueHeaders = map[string][]string(r.HTTPHeader)

	return json.Marshal(r.APIGatewayProxyResponse)
}

// Invoke invokes the API Gateway API call.
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
// handlers.
type ResourceHandler interface {
	ServeResource(context.Context, APIGatewayProxyRequest) (APIGatewayProxyResponse, error)
}

// ServeResource is an API Gateway Proxy Lambda multiplexer. Matches resources
// by exact name.
type ServeResource struct {
	resources map[string]ResourceHandler
}

// NewServeResource returns a ServeResource API Gateway resource.
func NewServeResource() *ServeResource {
	return &ServeResource{resources: map[string]ResourceHandler{}}
}

// ServeResource invokes the handler for the resource API Gateway resource.
func (s *ServeResource) ServeResource(
	ctx context.Context, req APIGatewayProxyRequest,
) (resp APIGatewayProxyResponse, err error) {
	h, ok := s.resources[req.Resource]
	if !ok {
		return resp, fmt.Errorf("resource handler not found for %s", req.Resource)
	}
	return h.ServeResource(ctx, req)
}

// Handle adds a new resource handler.
func (s *ServeResource) Handle(resource string, handler ResourceHandler) *ServeResource {
	s.resources[resource] = handler
	return s
}

// ServeMethod is an API Gateway Proxy Lambda multiplexer for request HTTP
// methods. Matches resources by exact name.
type ServeMethod struct {
	methods map[string]ResourceHandler
}

// NewServeMethod returns a ServeMethod, HTTP methods can be added.
func NewServeMethod() *ServeMethod {
	return &ServeMethod{methods: map[string]ResourceHandler{}}
}

// ServeResource invokes the handler for the resource HTTP method.
func (s *ServeMethod) ServeResource(
	ctx context.Context, req APIGatewayProxyRequest,
) (resp APIGatewayProxyResponse, err error) {
	h, ok := s.methods[req.HTTPMethod]
	if !ok {
		return resp, fmt.Errorf("method handler not found for %s:%s", req.Resource, req.HTTPMethod)
	}
	return h.ServeResource(ctx, req)
}

// Handle adds a new HTTP method handler.
func (s *ServeMethod) Handle(method string, handler ResourceHandler) *ServeMethod {
	s.methods[method] = handler

	return s
}

// ResourceHandlerFunc provides wrapping of a handler function as the
// ResourceHandler type.
type ResourceHandlerFunc func(context.Context, APIGatewayProxyRequest) (resp APIGatewayProxyResponse, err error)

// ServeResource invokes the handler for resource handler function.
func (f ResourceHandlerFunc) ServeResource(
	ctx context.Context, req APIGatewayProxyRequest,
) (resp APIGatewayProxyResponse, err error) {
	return f(ctx, req)
}
