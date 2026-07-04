// Package main demonstrates auto-validation on extract: setting
// espresso.SetDefaultValidator(validator.AsDefaultValidator()) makes every
// built-in extractor reject malformed payloads with a structured 400 JSON
// response BEFORE the handler runs.
//
// Run:
//
//	go run ./cmd/example/validate
//
// Then:
//
//	curl -i -XPOST localhost:38081/users -d '{"name":"al","email":"alice@example.com","role":"admin"}'
//	# 400 Bad Request — name is too short (validate:"min=3")
//
//	curl -i -XPOST localhost:38081/users -d '{"name":"alice","email":"not-an-email","role":"admin"}'
//	# 400 Bad Request — email fails the email rule
//
//	curl -i -XPOST localhost:38081/users -d '{"name":"alice","email":"alice@example.com","role":"admin"}'
//	# 200 OK
package main

import (
	"context"
	"net/http"

	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/validator"
)

// CreateUserReq is the request payload; its `validate:"..."` tags drive the
// auto-validation. With SetDefaultValidator(validator.AsDefaultValidator())
// installed below, every extraction of *espresso.JSON[CreateUserReq] runs
// validator.Struct on the decoded value and rejects on first failure.
type CreateUserReq struct {
	Name  string `json:"name"  validate:"required,min=3,max=50"`
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role"  validate:"required,oneof=admin user guest"`
}

func init() {
	// Wire validator.AsDefaultValidator() as the global hook. After this,
	// every built-in extractor (JSON, Query, Path, Form, Header, Cookie,
	// XML, Multipart, RawBodyWithHeaders) auto-validates after decode and
	// rejects failures with the framework's standard 400 JSON shape.
	espresso.SetDefaultValidator(validator.AsDefaultValidator())
}

func createUser(_ context.Context, req *espresso.JSON[CreateUserReq]) (espresso.JSON[response], error) {
	// Validation already ran. req.Data is known good.
	return espresso.JSON[response]{
		StatusCode: http.StatusOK,
		Data:       response{Created: req.Data.Name + "@" + req.Data.Role},
	}, nil
}

type response struct {
	Created string `json:"created"`
}

func main() {
	espresso.Portafilter().
		Post("/users", espresso.Doppio(createUser)).
		Brew(espresso.WithAddr(":38081"))
}
