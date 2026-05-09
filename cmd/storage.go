package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/daptin/daptin-cli/client"
	"github.com/daptin/daptin-cli/render"
	daptinClient "github.com/daptin/daptin-go-client"
	"github.com/urfave/cli/v2"
)

func storageCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "storage",
		Usage: "Manage Daptin cloud stores and storage actions",
		Subcommands: []*cli.Command{
			storageAddCommand(appCtx),
			storageListCommand(appCtx),
			storageRemoveCommand(appCtx),
			storageUploadCommand(appCtx),
			storageMkdirCommand(appCtx),
			storageRemovePathCommand(appCtx),
			storageMoveCommand(appCtx),
			storageUnsupportedCommand("ls", "direct cloud_store listing is not exposed by Daptin; use site list_files for site storage paths"),
			storageUnsupportedCommand("download", "direct cloud_store download is not exposed by Daptin; use site get_file or /asset routes for asset columns"),
		},
	}
}

func storageAddCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Create a cloud_store and optional rclone credential",
		ArgsUsage: "<name>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "type", Value: "s3", Usage: "Rclone storage type"},
			&cli.StringFlag{Name: "provider", Usage: "Rclone credential provider name"},
			&cli.StringFlag{Name: "store-provider", Usage: "cloud_store store_provider value, defaults to --type"},
			&cli.StringFlag{Name: "endpoint", Usage: "Storage endpoint"},
			&cli.StringFlag{Name: "access-key", Usage: "Access key id"},
			&cli.StringFlag{Name: "secret-key", Usage: "Secret access key"},
			&cli.StringFlag{Name: "bucket", Usage: "Bucket/container name"},
			&cli.StringFlag{Name: "root-path", Usage: "Full Daptin root_path, overrides name:bucket"},
			&cli.StringFlag{Name: "credential", Usage: "Existing credential name to link instead of creating one"},
			&cli.StringSliceFlag{Name: "param", Usage: "Additional rclone credential key=value"},
			&cli.BoolFlag{Name: "restart", Usage: "Execute world.restart_daptin after setup"},
		},
		Action: func(c *cli.Context) error {
			name := c.Args().Get(0)
			if name == "" {
				return fmt.Errorf("usage: storage add <name>")
			}

			credentialName := c.String("credential")
			var credentialRef string
			if credentialName == "" && hasCredentialInput(c) {
				credentialName = name
				created, err := createStorageCredential(appCtx, credentialName, storageCredentialContent(c))
				if err != nil {
					return err
				}
				credentialRef = refID(created)
			}
			if credentialName != "" && credentialRef == "" {
				cred, err := findOneByName(appCtx, "credential", credentialName)
				if err != nil {
					return err
				}
				credentialRef = refID(cred)
				if credentialRef == "" {
					return fmt.Errorf("credential %q has no reference_id", credentialName)
				}
			}

			rootPath := c.String("root-path")
			if rootPath == "" {
				rootPath = storageRootPath(name, c.String("bucket"))
			}
			if rootPath == "" {
				return fmt.Errorf("--bucket or --root-path required")
			}

			storeAttrs := map[string]interface{}{
				"name":             name,
				"store_type":       c.String("type"),
				"store_provider":   storageStoreProvider(c),
				"root_path":        rootPath,
				"credential_name":  credentialName,
				"store_parameters": "{}",
			}
			store, err := appCtx.Client.Create("cloud_store", jsonAPIObject("cloud_store", storeAttrs, ""))
			if err != nil {
				return err
			}

			storeRef := refID(store)
			if storeRef != "" && credentialRef != "" {
				if err := appCtx.Client.AddRelation("cloud_store", storeRef, "credential_id", "credential", credentialRef); err != nil {
					return err
				}
			} else if credentialName != "" {
				return fmt.Errorf("cloud_store created but credential %q could not be linked: missing reference_id", credentialName)
			}
			if c.Bool("restart") {
				responses, err := appCtx.Client.Execute("restart_daptin", "world", daptinClient.JsonApiObject{})
				if err != nil {
					return err
				}
				if err := applyEffects(ProcessResponses(responses), appCtx); err != nil {
					return err
				}
			}

			data, _ := store["attributes"].(map[string]interface{})
			if data == nil {
				data = store
			}
			if credentialName != "" {
				data["credential_name"] = credentialName
			}
			return appCtx.Renderer.RenderObject(data)
		},
	}
}

func storageListCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List configured cloud stores",
		Action: func(c *cli.Context) error {
			result, err := appCtx.Client.FindAll("cloud_store", daptinClient.DaptinQueryParameters{"page[size]": 500})
			if err != nil {
				return err
			}
			rows := render.FilterColumns(client.MapArray(result, "attributes"), []string{"name", "store_type", "store_provider", "root_path", "credential_name", "reference_id"})
			return appCtx.Renderer.RenderArray(rows)
		},
	}
}

func storageRemoveCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Usage:     "Delete a cloud_store by name or reference_id",
		ArgsUsage: "<name-or-reference-id>",
		Action: func(c *cli.Context) error {
			ref, err := cloudStoreRef(appCtx, c.Args().Get(0))
			if err != nil {
				return err
			}
			if err := appCtx.Client.Delete("cloud_store", ref); err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "Deleted")
			return nil
		},
	}
}

func storageUploadCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "upload",
		Usage:     "Upload a file or recursive directory to cloud_store via upload_file",
		ArgsUsage: "<store:/path> <local-path>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "recursive", Usage: "Upload a directory recursively"},
		},
		Action: func(c *cli.Context) error {
			address, localPath := c.Args().Get(0), c.Args().Get(1)
			if address == "" || localPath == "" {
				return fmt.Errorf("usage: storage upload <store:/path> <local-path>")
			}
			storeName, destPath, err := parseStorageAddress(address)
			if err != nil {
				return err
			}
			files, actionPath, err := buildUploadFiles(localPath, destPath, c.Bool("recursive"))
			if err != nil {
				return err
			}
			return executeCloudStoreAction(appCtx, storeName, "upload_file", map[string]interface{}{
				"path": actionPath,
				"file": files,
			})
		},
	}
}

func storageMkdirCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "mkdir",
		Usage:     "Create a folder in cloud_store",
		ArgsUsage: "<store:/path>",
		Action: func(c *cli.Context) error {
			storeName, targetPath, err := parseStorageAddress(c.Args().Get(0))
			if err != nil {
				return err
			}
			parent, name := splitRemoteParentName(targetPath)
			if name == "" {
				return fmt.Errorf("folder name required")
			}
			return executeCloudStoreAction(appCtx, storeName, "create_folder", map[string]interface{}{"path": parent, "name": name})
		},
	}
}

func storageRemovePathCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "Delete a path from cloud_store",
		ArgsUsage: "<store:/path>",
		Action: func(c *cli.Context) error {
			storeName, targetPath, err := parseStorageAddress(c.Args().Get(0))
			if err != nil {
				return err
			}
			return executeCloudStoreAction(appCtx, storeName, "delete_path", map[string]interface{}{"path": targetPath})
		},
	}
}

func storageMoveCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "mv",
		Usage:     "Move or rename a path in the same cloud_store",
		ArgsUsage: "<store:/source> <store:/destination>",
		Action: func(c *cli.Context) error {
			srcStore, source, err := parseStorageAddress(c.Args().Get(0))
			if err != nil {
				return err
			}
			dstStore, destination, err := parseStorageAddress(c.Args().Get(1))
			if err != nil {
				return err
			}
			if srcStore != dstStore {
				return fmt.Errorf("mv requires source and destination in the same store")
			}
			return executeCloudStoreAction(appCtx, srcStore, "move_path", map[string]interface{}{"source": source, "destination": destination})
		},
	}
}

func storageUnsupportedCommand(name, reason string) *cli.Command {
	return &cli.Command{
		Name:  name,
		Usage: reason,
		Action: func(c *cli.Context) error {
			return fmt.Errorf("%s is not supported for cloud_store: %s", name, reason)
		},
	}
}

func assetCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "asset",
		Usage: "Manage file asset columns",
		Subcommands: []*cli.Command{
			assetUploadCommand(appCtx),
			assetListCommand(appCtx),
		},
	}
}

func assetUploadCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "upload",
		Usage:     "Stream a file into a file.* asset column",
		ArgsUsage: "<entity> <reference_id> <column> <local-file>",
		Action: func(c *cli.Context) error {
			entityName, referenceID, columnName, localPath := c.Args().Get(0), c.Args().Get(1), c.Args().Get(2), c.Args().Get(3)
			if entityName == "" || referenceID == "" || columnName == "" || localPath == "" {
				return fmt.Errorf("usage: asset upload <entity> <reference_id> <column> <local-file>")
			}
			info, err := os.Stat(localPath)
			if err != nil {
				return err
			}
			if info.IsDir() {
				return fmt.Errorf("asset upload expects a file, got directory %q", localPath)
			}
			file, err := os.Open(localPath)
			if err != nil {
				return err
			}
			defer file.Close()

			filename := filepath.Base(localPath)
			contentType := contentTypeForPath(localPath)
			upload, err := appCtx.Client.UploadAssetStream(entityName, referenceID, columnName, filename, contentType, info.Size(), file)
			if err != nil {
				return err
			}
			uploadID, _ := upload["upload_id"].(string)
			complete, err := appCtx.Client.CompleteAssetUpload(entityName, referenceID, columnName, filename, uploadID, contentType, info.Size())
			if err != nil {
				return err
			}
			return appCtx.Renderer.RenderObject(complete)
		},
	}
}

func assetListCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List files recorded in a file.* asset column",
		ArgsUsage: "<entity> <reference_id> <column>",
		Action: func(c *cli.Context) error {
			entityName, referenceID, columnName := c.Args().Get(0), c.Args().Get(1), c.Args().Get(2)
			if entityName == "" || referenceID == "" || columnName == "" {
				return fmt.Errorf("usage: asset list <entity> <reference_id> <column>")
			}
			row, err := appCtx.Client.FindOne(entityName, referenceID, nil)
			if err != nil {
				return err
			}
			attrs, _ := row["attributes"].(map[string]interface{})
			if attrs == nil {
				return fmt.Errorf("no attributes returned")
			}
			files, ok := attrs[columnName].([]interface{})
			if !ok {
				if fileMaps, ok := attrs[columnName].([]map[string]interface{}); ok {
					rows := make([]map[string]interface{}, 0, len(fileMaps))
					rows = append(rows, fileMaps...)
					return appCtx.Renderer.RenderArray(rows)
				}
				return fmt.Errorf("column %q has no file list", columnName)
			}
			rows := make([]map[string]interface{}, 0, len(files))
			for _, file := range files {
				if row, ok := file.(map[string]interface{}); ok {
					rows = append(rows, row)
				}
			}
			return appCtx.Renderer.RenderArray(rows)
		},
	}
}

func executeCloudStoreAction(appCtx *AppContext, storeName, actionName string, attrs map[string]interface{}) error {
	ref, err := cloudStoreRef(appCtx, storeName)
	if err != nil {
		return err
	}
	attrs["cloud_store_id"] = ref
	responses, err := appCtx.Client.Execute(actionName, "cloud_store", daptinClient.JsonApiObject(attrs))
	if err != nil {
		return err
	}
	effects := ProcessResponses(responses)
	if len(effects) == 0 {
		effects = append(effects, BuildActionSuccessEffect("cloud_store", actionName, ref))
	}
	return applyEffects(effects, appCtx)
}

func cloudStoreRef(appCtx *AppContext, nameOrRef string) (string, error) {
	if nameOrRef == "" {
		return "", fmt.Errorf("cloud store name or reference_id required")
	}
	store, err := findOneByName(appCtx, "cloud_store", nameOrRef)
	if err == nil {
		if ref := refID(store); ref != "" {
			return ref, nil
		}
	}
	if strings.Contains(nameOrRef, "-") {
		return nameOrRef, nil
	}
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("cloud_store %q has no reference_id", nameOrRef)
}

func findOneByName(appCtx *AppContext, entityName, name string) (map[string]interface{}, error) {
	clauses, err := ParseFilter("name=" + name)
	if err != nil {
		return nil, err
	}
	result, err := appCtx.Client.FindAll(entityName, daptinClient.DaptinQueryParameters{
		"page[size]": 1,
		"query":      FilterToJSON(clauses),
	})
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%s %q not found", entityName, name)
	}
	return result[0], nil
}

func createStorageCredential(appCtx *AppContext, name string, content map[string]interface{}) (map[string]interface{}, error) {
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}
	return appCtx.Client.Create("credential", jsonAPIObject("credential", map[string]interface{}{
		"name":    name,
		"content": string(contentJSON),
	}, ""))
}

func jsonAPIObject(entityName string, attrs map[string]interface{}, id string) daptinClient.JsonApiObject {
	data := map[string]interface{}{
		"type":       entityName,
		"attributes": attrs,
	}
	if id != "" {
		data["id"] = id
	}
	return daptinClient.JsonApiObject{"data": data}
}

func refID(obj map[string]interface{}) string {
	if attrs, ok := obj["attributes"].(map[string]interface{}); ok {
		if ref, ok := attrs["reference_id"].(string); ok {
			return ref
		}
	}
	if ref, ok := obj["id"].(string); ok {
		return ref
	}
	if ref, ok := obj["reference_id"].(string); ok {
		return ref
	}
	return ""
}

func hasCredentialInput(c *cli.Context) bool {
	return c.String("access-key") != "" || c.String("secret-key") != "" || c.String("endpoint") != "" || c.String("provider") != "" || len(c.StringSlice("param")) > 0
}

func storageCredentialContent(c *cli.Context) map[string]interface{} {
	content := map[string]interface{}{
		"type": c.String("type"),
	}
	if c.String("provider") != "" {
		content["provider"] = c.String("provider")
	}
	if c.String("type") == "s3" {
		if _, ok := content["env_auth"]; !ok {
			content["env_auth"] = "false"
		}
		if _, ok := content["region"]; !ok {
			content["region"] = "us-east-1"
		}
	}
	if c.String("endpoint") != "" {
		content["endpoint"] = c.String("endpoint")
	}
	if c.String("access-key") != "" {
		content["access_key_id"] = c.String("access-key")
	}
	if c.String("secret-key") != "" {
		content["secret_access_key"] = c.String("secret-key")
	}
	for _, param := range c.StringSlice("param") {
		parts := strings.SplitN(param, "=", 2)
		if len(parts) == 2 {
			content[parts[0]] = parts[1]
		}
	}
	return content
}

func storageStoreProvider(c *cli.Context) string {
	if c.String("store-provider") != "" {
		return c.String("store-provider")
	}
	return c.String("type")
}

func storageRootPath(name, bucket string) string {
	if bucket == "" {
		return ""
	}
	return name + ":" + strings.TrimPrefix(bucket, "/")
}

func parseStorageAddress(address string) (string, string, error) {
	parts := strings.SplitN(address, ":", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", "", fmt.Errorf("expected storage address as store:/path")
	}
	path := parts[1]
	if path == "" {
		path = "/"
	}
	return parts[0], path, nil
}

func splitRemoteParentName(targetPath string) (string, string) {
	cleaned := strings.TrimRight(targetPath, "/")
	if cleaned == "" {
		return "", ""
	}
	parent := filepath.ToSlash(filepath.Dir(cleaned))
	name := filepath.Base(cleaned)
	if parent == "." || parent == "/" {
		parent = ""
	}
	return strings.TrimPrefix(parent, "/"), name
}

func buildUploadFiles(localPath, destPath string, recursive bool) ([]map[string]interface{}, string, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return nil, "", err
	}
	if info.IsDir() {
		if !recursive {
			return nil, "", fmt.Errorf("directory upload requires --recursive")
		}
		files := make([]map[string]interface{}, 0)
		err := filepath.WalkDir(localPath, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(localPath, path)
			if err != nil {
				return err
			}
			file, err := fileUploadObject(path, filepath.ToSlash(rel))
			if err != nil {
				return err
			}
			files = append(files, file)
			return nil
		})
		return files, normalizeRemoteDir(destPath), err
	}

	actionPath := normalizeRemoteDir(destPath)
	filename := filepath.Base(localPath)
	if !strings.HasSuffix(destPath, "/") {
		base := filepath.Base(destPath)
		if base != "." && base != "/" && base != "" {
			filename = base
			actionPath = normalizeRemoteDir(filepath.Dir(destPath))
		}
	}
	file, err := fileUploadObject(localPath, filename)
	if err != nil {
		return nil, "", err
	}
	return []map[string]interface{}{file}, actionPath, nil
}

func fileUploadObject(localPath, name string) (map[string]interface{}, error) {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return nil, err
	}
	contentType := contentTypeForPath(localPath)
	return map[string]interface{}{
		"name": name,
		"type": contentType,
		"file": "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data),
	}, nil
}

func normalizeRemoteDir(path string) string {
	if path == "" || path == "." || path == "/" {
		return ""
	}
	normalized := filepath.ToSlash(filepath.Clean(path))
	return strings.TrimPrefix(normalized, "/")
}

func contentTypeForPath(path string) string {
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}
