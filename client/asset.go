package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
)

// UploadAssetStream streams a file to Daptin's asset upload endpoint.
func (e *ExtendedClient) UploadAssetStream(entityName, referenceID, columnName, filename, contentType string, size int64, body io.Reader) (map[string]interface{}, error) {
	u := fmt.Sprintf("%s/asset/%s/%s/%s/upload?operation=stream&filename=%s",
		e.Endpoint,
		url.PathEscape(entityName),
		url.PathEscape(referenceID),
		url.PathEscape(columnName),
		url.QueryEscape(filename),
	)

	resp, err := e.nextRequest().
		SetHeader("Content-Type", contentType).
		SetHeader("X-File-Type", contentType).
		SetHeader("X-File-Size", fmt.Sprintf("%d", size)).
		SetBody(body).
		Post(u)
	if err := e.checkResponse(resp, err); err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &data); err != nil {
		return nil, fmt.Errorf("parse upload response: %w", err)
	}
	return data, nil
}

// CompleteAssetUpload finalizes an asset upload and updates the row's file column.
func (e *ExtendedClient) CompleteAssetUpload(entityName, referenceID, columnName, filename, uploadID, contentType string, size int64) (map[string]interface{}, error) {
	u := fmt.Sprintf("%s/asset/%s/%s/%s/upload?operation=complete&upload_id=%s&filename=%s",
		e.Endpoint,
		url.PathEscape(entityName),
		url.PathEscape(referenceID),
		url.PathEscape(columnName),
		url.QueryEscape(uploadID),
		url.QueryEscape(filename),
	)

	body := map[string]interface{}{
		"fileName": filename,
		"size":     size,
		"type":     contentType,
	}
	resp, err := e.nextRequest().
		SetHeader("X-File-Type", contentType).
		SetHeader("X-File-Size", fmt.Sprintf("%d", size)).
		SetBody(body).
		Post(u)
	if err := e.checkResponse(resp, err); err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &data); err != nil {
		return nil, fmt.Errorf("parse complete response: %w", err)
	}
	return data, nil
}
