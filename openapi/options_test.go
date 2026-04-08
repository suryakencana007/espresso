package openapi

import (
	"testing"
)

func TestSummary(t *testing.T) {
	op := &Operation{}
	Summary("List all users")(op)

	if op.Summary != "List all users" {
		t.Errorf("expected summary 'List all users', got %s", op.Summary)
	}
}

func TestDescription(t *testing.T) {
	op := &Operation{}
	Description("Returns a paginated list of users")(op)

	if op.Description != "Returns a paginated list of users" {
		t.Errorf("expected description 'Returns a paginated list of users', got %s", op.Description)
	}
}

func TestTags(t *testing.T) {
	op := &Operation{}
	Tags("users", "admin")(op)

	if len(op.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(op.Tags))
		return
	}

	if op.Tags[0] != "users" {
		t.Errorf("expected first tag 'users', got %s", op.Tags[0])
	}
	if op.Tags[1] != "admin" {
		t.Errorf("expected second tag 'admin', got %s", op.Tags[1])
	}
}

func TestTags_Append(t *testing.T) {
	op := &Operation{Tags: []string{"existing"}}
	Tags("users")(op)

	if len(op.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(op.Tags))
		return
	}

	if op.Tags[0] != "existing" {
		t.Errorf("expected first tag 'existing', got %s", op.Tags[0])
	}
	if op.Tags[1] != "users" {
		t.Errorf("expected second tag 'users', got %s", op.Tags[1])
	}
}

func TestSecurity(t *testing.T) {
	op := &Operation{}
	Security("bearerAuth", "apiKey")(op)

	if len(op.Security) != 2 {
		t.Errorf("expected 2 security schemes, got %d", len(op.Security))
		return
	}

	if _, ok := op.Security[0]["bearerAuth"]; !ok {
		t.Error("expected bearerAuth in security")
	}
	if _, ok := op.Security[1]["apiKey"]; !ok {
		t.Error("expected apiKey in security")
	}
}

func TestStatus(t *testing.T) {
	op := &Operation{}
	Status("201", Response{
		Description: "User created",
	})(op)

	if op.Responses == nil {
		t.Error("expected responses to be initialized")
		return
	}

	if op.Responses["201"].Description != "User created" {
		t.Errorf("expected '201' response description 'User created', got %s", op.Responses["201"].Description)
	}
}

func TestStatus_NilResponses(t *testing.T) {
	op := &Operation{}
	Status("200", Response{Description: "OK"})(op)

	if op.Responses == nil {
		t.Error("expected responses to be initialized")
	}
}

func TestDeprecated_WithDescription(t *testing.T) {
	op := &Operation{Description: "Get users"}
	Deprecated()(op)

	if op.Description != "Get users (Deprecated)" {
		t.Errorf("expected description 'Get users (Deprecated)', got %s", op.Description)
	}
}

func TestDeprecated_WithoutDescription(t *testing.T) {
	op := &Operation{}
	Deprecated()(op)

	if op.Description != "Deprecated" {
		t.Errorf("expected description 'Deprecated', got %s", op.Description)
	}
}

func TestAddParam(t *testing.T) {
	op := &Operation{}
	AddParam("q", "query", true, &Schema{Type: "string"})(op)

	if len(op.Parameters) != 1 {
		t.Errorf("expected 1 parameter, got %d", len(op.Parameters))
		return
	}

	param := op.Parameters[0]
	if param.Name != "q" {
		t.Errorf("expected parameter name 'q', got %s", param.Name)
	}
	if param.In != "query" {
		t.Errorf("expected parameter in 'query', got %s", param.In)
	}
	if !param.Required {
		t.Error("expected parameter to be required")
	}
	if param.Schema.Type != "string" {
		t.Errorf("expected schema type 'string', got %s", param.Schema.Type)
	}
}

func TestAddResponse(t *testing.T) {
	op := &Operation{}
	AddResponse("200", Response{
		Description: "Success",
	})(op)

	if op.Responses == nil {
		t.Error("expected responses to be initialized")
		return
	}

	if op.Responses["200"].Description != "Success" {
		t.Errorf("expected '200' response description 'Success', got %s", op.Responses["200"].Description)
	}
}

func TestAddResponse_Multiple(t *testing.T) {
	op := &Operation{}
	AddResponse("200", Response{Description: "OK"})(op)
	AddResponse("201", Response{Description: "Created"})(op)

	if len(op.Responses) != 2 {
		t.Errorf("expected 2 responses, got %d", len(op.Responses))
		return
	}

	if op.Responses["200"].Description != "OK" {
		t.Errorf("expected '200' response description 'OK', got %s", op.Responses["200"].Description)
	}
	if op.Responses["201"].Description != "Created" {
		t.Errorf("expected '201' response description 'Created', got %s", op.Responses["201"].Description)
	}
}

func TestApplyOptions(t *testing.T) {
	op := &Operation{}
	ApplyOptions(op,
		Summary("List users"),
		Tags("users"),
		Description("Returns list of users"),
	)

	if op.Summary != "List users" {
		t.Errorf("expected summary 'List users', got %s", op.Summary)
	}
	if len(op.Tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(op.Tags))
	}
	if op.Description != "Returns list of users" {
		t.Errorf("expected description 'Returns list of users', got %s", op.Description)
	}
}

func TestMultipleOptions(t *testing.T) {
	op := &Operation{}

	Summary("Create user")(op)
	Description("Creates a new user")(op)
	Tags("users", "admin")(op)
	Security("bearerAuth")(op)
	Status("201", Response{Description: "User created"})(op)

	if op.Summary != "Create user" {
		t.Errorf("expected summary 'Create user', got %s", op.Summary)
	}
	if op.Description != "Creates a new user" {
		t.Errorf("expected description 'Creates a new user', got %s", op.Description)
	}
	if len(op.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(op.Tags))
	}
	if len(op.Security) != 1 {
		t.Errorf("expected 1 security scheme, got %d", len(op.Security))
	}
	if op.Responses["201"].Description != "User created" {
		t.Errorf("expected '201' response to be set")
	}
}
