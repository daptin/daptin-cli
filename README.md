# Daptin CLI

CLI for talking to [Daptin server](https://github.com/daptin/daptin)

```bash
go get github.com/daptin/daptin-cli
go install github.com/daptin/daptin-cli
go build -o daptin-cli
```


Describe a table schema
```bash
./daptin-cli --output table describe table world
```

List items of a table

    ./daptin-cli list --pageSize 50 --columns <col1>,<col2> <tableName>

```bash
./daptin-cli list --pageSize 50 --columns reference_id,table_name world
```


Get one row by reference_id

    ./daptin-cli get-by-id --columns reference_id,table_name <table_name> <reference_id>

```bash
./daptin-cli get-by-id --columns reference_id,table_name,is_top_level world 019228bb-a7cd-773b-a465-c92d7c54d956 
```

