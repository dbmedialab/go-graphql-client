package graphql

import (
	"context"
	"net/http"

	"github.com/dbmedialab/go-graphql-client/internal/jsonutil"
)

// Client is a GraphQL client.
type Client struct {
	transport Transport
}

// NewClient creates a GraphQL client targeting the specified GraphQL server URL.
// If httpClient is nil, then http.DefaultClient is used.
func NewClient(url string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		transport: TransportHTTP{
			URL:        url,
			HTTPClient: httpClient,
		},
	}
}

// NewPluggableClient creates a GraphQL client using the transport implementation given.
// This is like NewClient, but can support any implementation, rather than just http.
// (This may also be useful for testing -- you can provide a transport which uses
// fixture data on the filesystem, for example!)
func NewPluggableClient(transport Transport) *Client {
	return &Client{
		transport: transport,
	}
}

// Query executes a single GraphQL query request,
// with a query derived from q, populating the response into it.
// q should be a pointer to struct that corresponds to the GraphQL schema.
func (c *Client) Query(ctx context.Context, q interface{}, variables map[string]interface{}) error {
	return c.do(ctx, q, constructQuery(q, variables), variables)
}

// QueryCustom executes a single GraphQL query request,
// with the query provided as a string, populating the response into q.
// slot should be a pointer to struct that corresponds to the GraphQL schema,
// and the variables in the query must be provided by the variables map.
func (c *Client) QueryCustom(ctx context.Context, q interface{}, query string, variables map[string]interface{}) error {
	return c.do(ctx, q, query, variables)
}

// Mutate executes a single GraphQL mutation request,
// with a mutation derived from m, populating the response into it.
// m should be a pointer to struct that corresponds to the GraphQL schema.
func (c *Client) Mutate(ctx context.Context, m interface{}, variables map[string]interface{}) error {
	return c.do(ctx, m, constructMutation(m, variables), variables)
}

// MutateCustom executes a single GraphQL mutation request,
// with the query provided as a string, populating the response into m.
// m should be a pointer to struct that corresponds to the GraphQL schema,
// and the variables in the query must be provided by the variables map.
func (c *Client) MutateCustom(ctx context.Context, m interface{}, query string, variables map[string]interface{}) error {
	return c.do(ctx, m, query, variables)
}

// do executes a single GraphQL operation.
func (c *Client) do(ctx context.Context, v interface{}, query string, variables map[string]interface{}) error {
	in := Request{
		Query:     query,
		Variables: variables,
	}

	out, err := c.transport.Do(ctx, in)
	err = jsonutil.UnmarshalGraphQL(out.Data, v)
	if err != nil {
		return err
	}
	if len(out.Errors) > 0 {
		return out.Errors
	}
	return nil
}

// errors represents the "errors" array in a response from a GraphQL server.
// If returned via error interface, the slice is expected to contain at least 1 element.
//
// Specification: https://facebook.github.io/graphql/#sec-Errors.
type errors []struct {
	Message   string
	Locations []struct {
		Line   int
		Column int
	}
}

// Error implements error interface.
func (e errors) Error() string {
	return e[0].Message
}
