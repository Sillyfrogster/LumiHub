package openapi

import (
	"fmt"
	"slices"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestContractPublishesThePublicAPIBase(t *testing.T) {
	document := contractDocument(t)
	servers := contractList(t, document["servers"], "servers")
	if len(servers) != 1 {
		t.Fatalf("servers = %v, want one public API base", servers)
	}
	server := contractMap(t, servers[0], "servers[0]")
	if server["url"] != "/api" {
		t.Errorf("server URL = %v, want /api", server["url"])
	}
}

func TestContractRequiresBrowserProofForCookieMutations(t *testing.T) {
	document := contractDocument(t)
	components := contractMap(t, document["components"], "components")
	parameters := contractMap(t, components["parameters"], "components.parameters")
	proof := contractMap(t, parameters["IllarinRequest"], "IllarinRequest")
	if proof["name"] != "X-Illarin-Request" || proof["in"] != "header" || proof["required"] != true {
		t.Fatalf("IllarinRequest = %v, want one required X-Illarin-Request header", proof)
	}
	proofSchema := contractMap(t, proof["schema"], "IllarinRequest.schema")
	values := contractList(t, proofSchema["enum"], "IllarinRequest.schema.enum")
	if len(values) != 1 || values[0] != "1" {
		t.Errorf("IllarinRequest values = %v, want only 1", values)
	}

	mutations := []struct {
		path      string
		method    string
		forbidden string
	}{
		{"/v1/link/requests/{userCode}/approve", "post", "Email is unverified or the browser origin/request proof is invalid"},
		{"/v1/link/requests/{userCode}/deny", "post", "Email is unverified or the browser origin/request proof is invalid"},
		{"/v1/link/authorizations/{requestCode}/approve", "post", "Email is unverified or the browser origin/request proof is invalid"},
		{"/v1/link/authorizations/{requestCode}/deny", "post", "Email is unverified or the browser origin/request proof is invalid"},
		{"/v1/instances/{id}", "delete", "The browser origin or request proof is invalid"},
		{"/v1/deliveries/{id}", "delete", "The browser origin or request proof is invalid"},
		{"/v1/assets/{id}/deliveries", "post", "The browser proof is invalid, or that instance cannot receive assets"},
	}
	paths := contractMap(t, document["paths"], "paths")
	for _, mutation := range mutations {
		t.Run(mutation.path, func(t *testing.T) {
			path := contractMap(t, paths[mutation.path], mutation.path)
			operation := contractMap(t, path[mutation.method], mutation.method)
			operationParameters := contractList(t, operation["parameters"], "parameters")
			if !hasReference(operationParameters, "#/components/parameters/IllarinRequest") {
				t.Error("operation does not require IllarinRequest")
			}
			responses := contractMap(t, operation["responses"], "responses")
			forbidden := contractMap(t, responses["403"], "responses.403")
			if forbidden["description"] != mutation.forbidden {
				t.Errorf("403 description = %v, want %q", forbidden["description"], mutation.forbidden)
			}
		})
	}
}

func TestContractBoundsLinkSecrets(t *testing.T) {
	document := contractDocument(t)
	components := contractMap(t, document["components"], "components")
	schemas := contractMap(t, components["schemas"], "components.schemas")
	wants := []struct {
		name string
		min  int
		max  int
	}{
		{"UserCode", 8, 16},
		{"RequestCode", 43, 43},
		{"DeviceCode", 43, 43},
		{"ApprovalToken", 43, 43},
		{"AuthorizationCode", 43, 43},
		{"RefreshToken", 56, 56},
	}
	for _, want := range wants {
		t.Run(want.name, func(t *testing.T) {
			schema := contractMap(t, schemas[want.name], want.name)
			if length(schema["minLength"]) != want.min || length(schema["maxLength"]) != want.max {
				t.Errorf("bounds = %v..%v, want %d..%d", schema["minLength"], schema["maxLength"], want.min, want.max)
			}
			if schema["pattern"] == nil {
				t.Error("schema has no character pattern")
			}
		})
	}
}

func TestContractDiscriminatesPendingAndLinkedPolls(t *testing.T) {
	document := contractDocument(t)
	components := contractMap(t, document["components"], "components")
	schemas := contractMap(t, components["schemas"], "components.schemas")
	result := contractMap(t, schemas["LinkPollResult"], "LinkPollResult")
	variants := contractList(t, result["oneOf"], "LinkPollResult.oneOf")
	if len(variants) != 2 ||
		!hasReference(variants, "#/components/schemas/PendingLinkPollResult") ||
		!hasReference(variants, "#/components/schemas/LinkedLinkPollResult") {
		t.Fatalf("poll variants = %v", variants)
	}
	discriminator := contractMap(t, result["discriminator"], "LinkPollResult.discriminator")
	if discriminator["propertyName"] != "status" {
		t.Errorf("poll discriminator = %v, want status", discriminator["propertyName"])
	}

	linked := contractMap(t, schemas["LinkedLinkPollResult"], "LinkedLinkPollResult")
	required := stringsFrom(contractList(t, linked["required"], "LinkedLinkPollResult.required"))
	for _, field := range []string{"status", "accessToken", "accessTokenExpiresAt", "refreshToken", "instance"} {
		if !slices.Contains(required, field) {
			t.Errorf("linked poll does not require %s", field)
		}
	}
}

func contractDocument(t *testing.T) map[string]any {
	t.Helper()
	var document map[string]any
	if err := yaml.Unmarshal(Contract, &document); err != nil {
		t.Fatalf("read embedded OpenAPI: %v", err)
	}
	return document
}

func contractMap(t *testing.T, value any, name string) map[string]any {
	t.Helper()
	found, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s = %T, want an object", name, value)
	}
	return found
}

func contractList(t *testing.T, value any, name string) []any {
	t.Helper()
	found, ok := value.([]any)
	if !ok {
		t.Fatalf("%s = %T, want a list", name, value)
	}
	return found
}

func hasReference(values []any, reference string) bool {
	for _, value := range values {
		object, ok := value.(map[string]any)
		if ok && object["$ref"] == reference {
			return true
		}
	}
	return false
}

func length(value any) int {
	var parsed int
	_, _ = fmt.Sscan(fmt.Sprint(value), &parsed)
	return parsed
}

func stringsFrom(values []any) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if ok {
			result = append(result, text)
		}
	}
	return result
}
