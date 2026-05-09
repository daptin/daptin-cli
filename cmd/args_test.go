package cmd

import (
	"reflect"
	"testing"
)

func TestReorderArgs_FlagsAfterPositional(t *testing.T) {
	input := []string{"daptin", "list", "usergroup", "--filter", "foo"}
	expected := []string{"daptin", "list", "--filter", "foo", "usergroup"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_FilterValueWithEquals(t *testing.T) {
	input := []string{"daptin", "list", "usergroup", "--filter", "name=users"}
	expected := []string{"daptin", "list", "--filter", "name=users", "usergroup"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_AlreadyCorrect(t *testing.T) {
	input := []string{"daptin", "list", "--filter", "foo", "usergroup"}
	expected := []string{"daptin", "list", "--filter", "foo", "usergroup"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_GlobalFlagsPreserved(t *testing.T) {
	input := []string{"daptin", "--output", "json", "list", "usergroup", "--page-size", "50"}
	expected := []string{"daptin", "--output", "json", "list", "--page-size", "50", "usergroup"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_ExecuteWithReferenceId(t *testing.T) {
	input := []string{"daptin", "execute", "oauth_connect", "oauth_login_begin", "--reference-id", "abc"}
	expected := []string{"daptin", "execute", "--reference-id", "abc", "oauth_connect", "oauth_login_begin"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_KeyValNotReordered(t *testing.T) {
	input := []string{"daptin", "execute", "user_account", "signin", "email=x", "password=y"}
	expected := []string{"daptin", "execute", "user_account", "signin", "email=x", "password=y"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_DescribeSubcommand(t *testing.T) {
	input := []string{"daptin", "describe", "table", "document", "--columns", "ColumnName"}
	expected := []string{"daptin", "describe", "table", "--columns", "ColumnName", "document"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_NoFlags(t *testing.T) {
	input := []string{"daptin", "context", "add", "local", "http://localhost:6336"}
	expected := []string{"daptin", "context", "add", "local", "http://localhost:6336"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_BoolFlag(t *testing.T) {
	input := []string{"daptin", "execute", "user_account", "signin", "--interactive", "email=x"}
	expected := []string{"daptin", "execute", "--interactive", "user_account", "signin", "email=x"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_MultipleFlagsAfterArg(t *testing.T) {
	input := []string{"daptin", "list", "usergroup", "--sort", "name", "--page-size", "50", "--page", "2"}
	expected := []string{"daptin", "list", "--sort", "name", "--page-size", "50", "--page", "2", "usergroup"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_SortDescendingValue(t *testing.T) {
	input := []string{"daptin", "list", "document", "--sort", "-created_at"}
	expected := []string{"daptin", "list", "--sort", "-created_at", "document"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_StorageAddFlagsAfterName(t *testing.T) {
	input := []string{"daptin", "storage", "add", "minio", "--type", "s3", "--endpoint", "http://localhost:9000"}
	expected := []string{"daptin", "storage", "add", "--type", "s3", "--endpoint", "http://localhost:9000", "minio"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_OAuthNestedSubcommand(t *testing.T) {
	input := []string{"daptin", "oauth", "connect", "create", "asana.com", "--client-id", "abc", "--client-secret-env", "ASANA_SECRET"}
	expected := []string{"daptin", "oauth", "connect", "create", "--client-id", "abc", "--client-secret-env", "ASANA_SECRET", "asana.com"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_IntegrationExecute(t *testing.T) {
	input := []string{"daptin", "integration", "execute", "asana.com", "getWorkspaces", "--oauth-token-id", "tok", "workspace=abc"}
	expected := []string{"daptin", "integration", "execute", "--oauth-token-id", "tok", "asana.com", "getWorkspaces", "workspace=abc"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_EqualsFlag(t *testing.T) {
	input := []string{"daptin", "list", "usergroup", "--page-size=50"}
	expected := []string{"daptin", "list", "--page-size=50", "usergroup"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_GlobalDebugFlag(t *testing.T) {
	input := []string{"daptin", "--debug", "list", "world", "--columns", "table_name"}
	expected := []string{"daptin", "--debug", "list", "--columns", "table_name", "world"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_HelpFlag(t *testing.T) {
	input := []string{"daptin", "list", "--help"}
	expected := []string{"daptin", "list", "--help"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_EntityNamedAction(t *testing.T) {
	// "action" is both a subcommand of "describe" and a valid entity name for "list"
	input := []string{"daptin", "list", "action", "--columns", "action_name", "--page-size", "3"}
	expected := []string{"daptin", "list", "--columns", "action_name", "--page-size", "3", "action"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_DescribeActionSubcommand(t *testing.T) {
	// "action" as an actual subcommand of "describe" should NOT be reordered
	input := []string{"daptin", "describe", "action", "document", "createDocument"}
	expected := []string{"daptin", "describe", "action", "document", "createDocument"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestReorderArgs_Empty(t *testing.T) {
	input := []string{"daptin"}
	expected := []string{"daptin"}

	result := ReorderArgs(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}
