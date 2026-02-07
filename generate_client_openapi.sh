go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest

curl http://localhost:6336/openapi.yaml > daptin-openapi.yaml

oapi-codegen --config=cfg.yaml client/daptin-openapi.yaml