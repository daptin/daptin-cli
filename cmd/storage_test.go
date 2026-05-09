package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseStorageAddress(t *testing.T) {
	store, path, err := parseStorageAddress("minio:/photos")
	if err != nil {
		t.Fatal(err)
	}
	if store != "minio" || path != "/photos" {
		t.Fatalf("unexpected address: %q %q", store, path)
	}
}

func TestParseStorageAddress_DefaultRootPath(t *testing.T) {
	store, path, err := parseStorageAddress("minio:")
	if err != nil {
		t.Fatal(err)
	}
	if store != "minio" || path != "/" {
		t.Fatalf("unexpected address: %q %q", store, path)
	}
}

func TestSplitRemoteParentName(t *testing.T) {
	parent, name := splitRemoteParentName("/photos/2026")
	if parent != "photos" || name != "2026" {
		t.Fatalf("unexpected parent/name: %q %q", parent, name)
	}
}

func TestBuildUploadFiles_RenameSingleFile(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(localPath, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}

	files, actionPath, err := buildUploadFiles(localPath, "/docs/output.txt", false)
	if err != nil {
		t.Fatal(err)
	}
	if actionPath != "docs" {
		t.Fatalf("expected action path docs, got %q", actionPath)
	}
	if len(files) != 1 || files[0]["name"] != "output.txt" {
		t.Fatalf("unexpected files: %#v", files)
	}
	if files[0]["type"] != "text/plain; charset=utf-8" {
		t.Fatalf("unexpected content type: %#v", files[0]["type"])
	}
}

func TestBuildUploadFiles_DirectoryRequiresRecursive(t *testing.T) {
	_, _, err := buildUploadFiles(t.TempDir(), "/docs/", false)
	if err == nil {
		t.Fatal("expected recursive error")
	}
}
