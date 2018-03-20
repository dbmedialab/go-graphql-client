package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/shurcooL/go/ctxhttp"
)

// Transport is an interface that can be implemented to replace the
// serialization and network mechanism used by the client.
//
// HTTP+JSON is the most common serialization and transport, and the default
// in this library, but you can define your own!
// There's no reason that GraphQL queries can't be issued over GRPC;
// mock transports that store and replay data may also be great for testing.
type Transport interface {
	Do(context.Context, Request) (*Response, error)
}

// Request is a type used by the Transport interface.  Users of the library
// don't need to use this type unless they're implementing a Transport.
//
// Request gathers all fields used in a graphql request (the query together
// with assignments of any variables) together for serialization.
type Request struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// Response is a type used by the Transport interface.  Users of the library
// don't need to use this type unless they're implementing a Transport.
//
// Response holds the main "data" of the response during the first phase of
// deserialization, and also recognizes any errors in the protocol response.
// (A second phase of deserialization maps the raw data into your go types;
// this is not handled by the Transport interface.)
type Response struct {
	Data   json.RawMessage
	Errors errors
	//Extensions interface{} // Unused.
}

var (
	_ Transport = TransportHTTP{}
	//_ Transport = TransportRecorder{}
	//_ Transport = TransportReplayer{}
)

type TransportHTTP struct {
	URL        string // GraphQL server URL.
	HTTPClient *http.Client
}

func (t TransportHTTP) Do(ctx context.Context, req Request) (*Response, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(req)
	if err != nil {
		return nil, err
	}
	resp, err := ctxhttp.Post(ctx, t.HTTPClient, t.URL, "application/json", &buf)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %v", resp.Status)
	}
	out := Response{}
	err = json.NewDecoder(resp.Body).Decode(&out)
	return &out, err
}
