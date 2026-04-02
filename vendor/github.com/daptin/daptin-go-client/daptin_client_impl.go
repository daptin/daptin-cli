package daptin_go_client

import (
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
)

type daptinClientImpl struct {
	endpoint    string
	authToken   string
	restyClient *resty.Client
	debug       bool
}

func (d daptinClientImpl) SetDebug(b bool) {
	d.debug = b
}

func (d daptinClientImpl) nextRequest() *resty.Request {
	request := d.restyClient.NewRequest().
		SetAuthToken(d.authToken).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json")
	if d.debug {
		request.EnableTrace()
	}

	return request
}

func (d daptinClientImpl) FindOne(tableName string, referenceId string, parameters DaptinQueryParameters) (JsonApiObject, error) {
	request := d.nextRequest()

	var responseObject JsonApiObject

	url := d.endpoint + "/api/" + tableName + "/" + referenceId
	if parameters != nil {
		for key, value := range parameters {
			url = url + key + "=" + fmt.Sprintf("%v", value) + "&"
		}
	}
	response, err := request.Get(url)

	if d.debug {
		d.LogTraceInfo(err, response)
	}
	if err != nil {
		return nil, err
	}

	bodyBytes := response.Body()

	err = json.Unmarshal(bodyBytes, &responseObject)

	return ToJsonApiObject(responseObject["data"].(map[string]interface{})), err

}

func (d daptinClientImpl) FindAll(tableName string, parameters DaptinQueryParameters) ([]JsonApiObject, error) {
	request := d.nextRequest()

	var responseObject JsonApiObject

	url := d.endpoint + "/api/" + tableName + "?"
	for key, parameter := range parameters {
		url = url + key + "=" + fmt.Sprintf("%v", parameter) + "&"
	}
	response, err := request.Get(url)

	if d.debug {
		d.LogTraceInfo(err, response)
	}
	if err != nil {
		return nil, err
	}

	bodyBytes := response.Body()

	err = json.Unmarshal(bodyBytes, &responseObject)

	return ToJsonApiObjectArray(responseObject["data"].([]interface{})), err

}

func ToJsonApiObjectArray(object []interface{}) []JsonApiObject {
	out := make([]JsonApiObject, len(object))
	for i, o := range object {
		out[i] = ToJsonApiObject(o.(map[string]interface{}))
	}
	return out
}

func ToJsonApiObject(object map[string]interface{}) JsonApiObject {
	return object
}

func (d daptinClientImpl) Create(tableName string, attributes JsonApiObject) (JsonApiObject, error) {

	request := d.nextRequest()

	var responseObject JsonApiObject

	response, err := request.SetBody(attributes).Post(d.endpoint + "/api/" + tableName)

	if d.debug {
		d.LogTraceInfo(err, response)
	}
	if err != nil {
		return nil, err
	}

	bodyBytes := response.Body()

	err = json.Unmarshal(bodyBytes, &responseObject)

	return ToJsonApiObject(responseObject["data"].(map[string]interface{})), err

}

func (d daptinClientImpl) Update(tableName, referenceId string, object JsonApiObject) (JsonApiObject, error) {
	request := d.nextRequest()

	var responseObject JsonApiObject

	response, err := request.SetBody(object).Patch(d.endpoint + "/api/" + tableName + "/" + referenceId)

	if d.debug {
		d.LogTraceInfo(err, response)
	}
	if err != nil {
		return nil, err
	}

	bodyBytes := response.Body()

	err = json.Unmarshal(bodyBytes, &responseObject)

	return ToJsonApiObject(responseObject["data"].(map[string]interface{})), err

}

func (d daptinClientImpl) Delete(tableName string, referenceId string) error {
	request := d.nextRequest()

	//var responseObject JsonApiObject

	response, err := request.Delete(d.endpoint + "/api/" + tableName + "/" + referenceId)

	if d.debug {
		d.LogTraceInfo(err, response)
	}
	return err
}

func (d daptinClientImpl) Execute(actionName string, tableName string, attributes JsonApiObject) ([]DaptinActionResponse, error) {
	request := d.nextRequest()

	var responseObject []DaptinActionResponse

	actionAttributes := map[string]interface{}{
		"Name":       actionName,
		"OnType":     tableName,
		"Attributes": attributes,
	}

	response, err := request.SetBody(actionAttributes).Post(d.endpoint + "/action/" + tableName + "/" + actionName)

	if d.debug {
		d.LogTraceInfo(err, response)
	}
	if err != nil {
		return nil, err
	}

	bodyBytes := response.Body()

	err = json.Unmarshal(bodyBytes, &responseObject)

	return responseObject, err

}

func (d daptinClientImpl) LogTraceInfo(err error, resp *resty.Response) {
	// Explore response object
	fmt.Println("Response Info:")
	fmt.Println("Error      :", err)
	fmt.Println("Status Code:", resp.StatusCode())
	fmt.Println("Status     :", resp.Status())
	fmt.Println("Proto      :", resp.Proto())
	fmt.Println("Time       :", resp.Time())
	fmt.Println("Received At:", resp.ReceivedAt())
	fmt.Println("Body       :\n", resp)
	fmt.Println()

	// Explore trace info
	fmt.Println("Request Trace Info:")
	ti := resp.Request.TraceInfo()
	fmt.Println("DNSLookup    :", ti.DNSLookup)
	fmt.Println("ConnTime     :", ti.ConnTime)
	fmt.Println("TCPConnTime  :", ti.TCPConnTime)
	fmt.Println("TLSHandshake :", ti.TLSHandshake)
	fmt.Println("ServerTime   :", ti.ServerTime)
	fmt.Println("ResponseTime :", ti.ResponseTime)
	fmt.Println("TotalTime    :", ti.TotalTime)
	fmt.Println("IsConnReused :", ti.IsConnReused)
	fmt.Println("IsConnWasIdle:", ti.IsConnWasIdle)
	fmt.Println("ConnIdleTime :", ti.ConnIdleTime)

}
