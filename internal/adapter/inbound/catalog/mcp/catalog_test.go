package mcp

import "testing"

func TestBuildCatalogPreservesDispatchAndOwnsItsSchemas(t *testing.T) {
	first, second := Build(), Build()
	found := false
	for _, tool := range first.Tools {
		if tool["name"] == "issueops_execution" {
			found = true
		}
	}
	if !found || first.Dispatch["issueops_execution"] != "issueops" {
		t.Fatal("public execution tool lost")
	}
	for _, tool := range first.Tools {
		if tool["name"] == "self_augment_history" {
			t.Fatal("unadvertised alias exposed")
		}
	}
	if first.Dispatch["self_augment_history"] != "self_loop" {
		t.Fatal("alias dispatch lost")
	}
	hasDocs := false
	for _, resource := range first.Resources {
		if resource["uri"] == "issueops://docs" {
			hasDocs = true
		}
	}
	if !hasDocs {
		t.Fatal("docs resource lost")
	}
	first.Tools[0]["inputSchema"].(map[string]any)["type"] = "broken"
	first.Dispatch["issueops_execution"] = "broken"
	first.Resources[0]["mimeType"] = "broken"
	if second.Tools[0]["inputSchema"].(map[string]any)["type"] != "object" || second.Dispatch["issueops_execution"] != "issueops" || second.Resources[0]["mimeType"] != "text/markdown" {
		t.Fatal("catalog instances share mutable state")
	}
}
