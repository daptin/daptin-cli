package main

import daptinClient "github.com/daptin/daptin-go-client"

func MapArray(allWorlds []daptinClient.JsonApiObject, keyName string) []map[string]interface{} {
	worlds := make([]map[string]interface{}, len(allWorlds))
	for i, w := range allWorlds {
		worlds[i] = w[keyName].(map[string]interface{})
	}
	return worlds
}
