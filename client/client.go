package client

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

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
	slog.Debug("creating client", "endpoint", endpoint, "token_present", authToken != "", "debug", debug)

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
	slog.Debug("preparing request", "auth_header_set", e.AuthToken != "")
	return req
}

func (e *ExtendedClient) checkResponse(resp *resty.Response, err error) error {
	if err != nil {
		return err
	}
	slog.Debug("response received", "status", resp.StatusCode())
	return CheckStatusCode(resp.StatusCode(), resp.String())
}

// FindOne overrides the upstream to fix the URL parameter bug
// (upstream appends params without a ? separator).
func (e *ExtendedClient) FindOne(tableName, referenceId string, parameters daptinClient.DaptinQueryParameters) (daptinClient.JsonApiObject, error) {
	u := BuildFindOneURL(e.Endpoint, tableName, referenceId, parameters)
	slog.Debug("FindOne", "url", u)

	resp, err := e.nextRequest().Get(u)
	if err := e.checkResponse(resp, err); err != nil {
		return nil, err
	}

	return ParseSingleResponse(resp.Body())
}

// FindAll overrides the upstream to handle unsupported/malformed API responses
// without panicking on JSON:API data shape assertions.
func (e *ExtendedClient) FindAll(tableName string, parameters daptinClient.DaptinQueryParameters) ([]daptinClient.JsonApiObject, error) {
	u := BuildFindAllURL(e.Endpoint, tableName, parameters)
	slog.Debug("FindAll", "url", u)

	resp, err := e.nextRequest().Get(u)
	if err := e.checkResponse(resp, err); err != nil {
		if errors.Is(err, ErrNotFound) && looksLikeJoinTable(tableName) {
			return nil, unsupportedJoinTableError(tableName)
		}
		return nil, err
	}

	items, err := ParseListResponse(resp.Body())
	if err != nil {
		return nil, err
	}
	if items == nil {
		if looksLikeJoinTable(tableName) {
			return nil, unsupportedJoinTableError(tableName)
		}
		return nil, fmt.Errorf("unexpected list response for %q: expected JSON:API data array", tableName)
	}

	result := make([]daptinClient.JsonApiObject, 0, len(items))
	for _, item := range items {
		result = append(result, daptinClient.JsonApiObject(item))
	}
	return result, nil
}

// Update overrides the upstream to handle error responses without panicking.
func (e *ExtendedClient) Update(tableName, referenceId string, object daptinClient.JsonApiObject) (daptinClient.JsonApiObject, error) {
	u := e.Endpoint + "/api/" + tableName + "/" + referenceId
	slog.Debug("Update", "url", u)
	resp, err := e.nextRequest().SetBody(object).Patch(u)
	if err := e.checkResponse(resp, err); err != nil {
		return nil, err
	}
	return ParseSingleResponse(resp.Body())
}

// Create overrides the upstream to handle error responses without panicking.
func (e *ExtendedClient) Create(tableName string, attributes daptinClient.JsonApiObject) (daptinClient.JsonApiObject, error) {
	u := e.Endpoint + "/api/" + tableName
	slog.Debug("Create", "url", u)
	resp, err := e.nextRequest().SetBody(attributes).Post(u)
	if err := e.checkResponse(resp, err); err != nil {
		return nil, err
	}
	return ParseSingleResponse(resp.Body())
}

// Delete overrides the upstream to add HTTP status checking.
func (e *ExtendedClient) Delete(tableName, referenceId string) error {
	u := e.Endpoint + "/api/" + tableName + "/" + referenceId
	slog.Debug("Delete", "url", u)
	resp, err := e.nextRequest().Delete(u)
	return e.checkResponse(resp, err)
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

func looksLikeJoinTable(tableName string) bool {
	return strings.Contains(tableName, "_has_")
}

func unsupportedJoinTableError(tableName string) error {
	return fmt.Errorf("%q looks like a generated join table. Daptin join tables are internal storage and are not exposed as API entities; use the owning API entity and relation/permission commands instead", tableName)
}
