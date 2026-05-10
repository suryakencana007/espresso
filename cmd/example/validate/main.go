// Package main demonstrates auto-validation on extract: setting
// espresso.SetDefaultValidator(validator.Struct) makes every built-in
// extractor reject malformed payloads with a structured 400 JSON
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

// CreateUserReq's `validate:"..."` tags drive the auto-validation. With
// SetDefaultValidator(validator.Struct) installed below, every extraction
// of *espresso.JSON[CreateUserReq] runs validator.Struct on the decoded
// value and rejects on first failure.
type CreateUserReq struct {
	Name  string `json:"name"  validate:"required,min=3,max=50"`
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role"  validate:"required,oneof=admin user guest"`
}

func init() {
	// Wire validator.Struct as the global hook. After this, every
	// built-in extractor (JSON, Query, Path, Form, Header, Cookie, XML,
	// Multipart, RawBodyWithHeaders) auto-validates after decode.
	espresso.SetDefaultValidator(structAdapter)
}

// structAdapter wraps validator.Struct to match the SetDefaultValidator
// signature (func(any) error). The wrapping is needed because v.Struct
// returns its own typed error; we surface it as-is so the framework's
// structured-error pipeline carries it through to the client as a 400.
func structAdapter(v any) error {
	if err := validator.Struct(v); err != nil {
		// validator.Struct returns espresso.FieldErrors on validation
		// failure; ValidationErrors wraps it in the standard 400 shape.
		if fe, ok := err.(espresso.FieldErrors); ok {
			return espresso.ValidationErrors(fe.ToValidationErrors())
		}
		return err
	}
	return nil
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
