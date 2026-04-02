package client

import (
	"errors"

	daptinClient "github.com/daptin/daptin-go-client"
	"github.com/go-resty/resty/v2"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

// ExtendedClient wraps the upstream DaptinClient and adds methods
// for API endpoints that the upstream library does not cover.
type ExtendedClient struct {
	daptinClient.DaptinClient
	Endpoint  string
	AuthToken string
	HTTP      *resty.Client
	Debug     bool
}

func New(endpoint, authToken string, debug bool) *ExtendedClient {
	var upstream daptinClient.DaptinClient
	if authToken == "" {
		upstream = daptinClient.NewDaptinClient(endpoint, debug)
	} else {
		upstream = daptinClient.NewDaptinClientWithAuthToken(endpoint, authToken, debug)
	}

	httpClient := resty.New()
	if debug {
		httpClient.SetDebug(true)
	}

	return &ExtendedClient{
		DaptinClient: upstream,
		Endpoint:     endpoint,
		AuthToken:    authToken,
		HTTP:         httpClient,
		Debug:        debug,
	}
}

func (e *ExtendedClient) nextRequest() *resty.Request {
	req := e.HTTP.NewRequest().
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json")
	if e.AuthToken != "" {
		req.SetAuthToken(e.AuthToken)
	}
	return req
}

func (e *ExtendedClient) checkResponse(resp *resty.Response, err error) error {
	if err != nil {
		return err
	}
	return CheckStatusCode(resp.StatusCode(), resp.String())
}

// FindOne overrides the upstream to fix the URL parameter bug
// (upstream appends params without a ? separator).
func (e *ExtendedClient) FindOne(tableName, referenceId string, parameters daptinClient.DaptinQueryParameters) (daptinClient.JsonApiObject, error) {
	u := BuildFindOneURL(e.Endpoint, tableName, referenceId, parameters)

	resp, err := e.nextRequest().Get(u)
	if err := e.checkResponse(resp, err); err != nil {
		return nil, err
	}

	return ParseSingleResponse(resp.Body())
}

// MapArray extracts a named sub-map from each JsonApiObject.
func MapArray(objects []daptinClient.JsonApiObject, keyName string) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(objects))
	for _, obj := range objects {
		if sub, ok := obj[keyName].(map[string]interface{}); ok {
			result = append(result, sub)
		}
	}
	return result
}
