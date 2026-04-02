package daptin_go_client

import "github.com/go-resty/resty/v2"

type JsonApiObject map[string]interface{}
type DaptinQueryParameters map[string]interface{}

type DaptinActionResponse struct {
	ResponseType string
	Attributes   map[string]interface{}
}

type DaptinClient interface {
	FindOne(tableName string, referenceId string, parameters DaptinQueryParameters) (JsonApiObject, error)
	FindAll(tableName string, parameters DaptinQueryParameters) ([]JsonApiObject, error)
	Create(tableName string, attributes JsonApiObject) (JsonApiObject, error)
	Update(tableName, referenceId string, object JsonApiObject) (JsonApiObject, error)
	Delete(tableName string, referenceId string) error
	Execute(actionName string, tableName string, attributes JsonApiObject) ([]DaptinActionResponse, error)
	SetDebug(bool)
}

type World struct {
	Name string
}

type DaptinClientHelper interface {
	GetAllWorld() []World
}

func NewDaptinClient(endpoint string, debug bool) DaptinClient {

	return &daptinClientImpl{
		endpoint:    endpoint,
		restyClient: resty.New(),
		debug:       debug,
	}
}

func NewDaptinClientWithAuthToken(endpoint string, authToken string, debug bool) DaptinClient {

	return &daptinClientImpl{
		restyClient: resty.New(),
		endpoint:    endpoint,
		authToken:   authToken,
		debug:       debug,
	}
}
