# openapi-godoc

[![Go Report Card](https://goreportcard.com/badge/github.com/tink3rlabs/openapi-godoc)](https://goreportcard.com/report/github.com/tink3rlabs/openapi-godoc)
[![Go Reference](https://pkg.go.dev/badge/github.com/tink3rlabs/openapi-godoc.svg)](https://pkg.go.dev/github.com/tink3rlabs/openapi-godoc)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Lint and Test](https://github.com/tink3rlabs/openapi-godoc/actions/workflows/build.yml/badge.svg)](https://github.com/tink3rlabs/openapi-godoc/actions/workflows/lint_and_test.yml)

This package generates an [OpenAPI](https://swagger.io/specification/v3/) document from comments in your code annotated with the `@openapi` keyword. It currently supports only the openapi 3.x schema and not the older swagger 2 schema.

## Goals

openapi-godoc enables you to integrate OpenAPI using comments in your code. Just add the `@openapi` keyword at the top of a `struct` or `func` declaration and describe the given API part in YAML syntax. It's also possible to pass JSON snippets directly outside the annotated source code.

openapi-godoc will then parse the comments and output an OpenAPI document describing your API, allowing you to keep your API documentation as close as possible to your code thus maximizing the likelihood it stays up to date when your code changes.

## Non-goals

openapi-godoc does not add logic to your specification or generate client code from the OpenAPI document. It is based on code annotations and/or static JSON, not the code logic itself. It works only with what you put around your logic, not the contents of the logic.

## Installation

```bash
go get github.com/tink3rlabs/openapi-godoc@latest
```

## Usage

Add comments to your `strucs` and `funcs` that represent API components. Then define your API by creating an `openapigodoc.OpenAPIDefinition` object and providing general information about your API and OpenAPI specification components.

```go
package main

import (
 "log"
 "net/http"

 "github.com/go-chi/chi"
 "github.com/go-chi/chi/middleware"
 "github.com/go-chi/cors"
 "github.com/go-chi/render"
 openapigodoc "github.com/tink3rlabs/openapi-godoc"
)

// @openapi
// components:
//
// schemas:
//   Message:
//     type: object
//     properties:
//       content:
//         type: string
//         description: The contents of a message
//         example: Hello world!
type Message struct {
 Content string `json:"content"`
}

// @openapi
// components:
//
// responses:
//   NotFound:
//     description: The specified resource was not found
//     content:
//       application/json:
//         schema:
//           $ref: '#/components/schemas/Error'
//   Unauthorized:
//     description: Unauthorized
//     content:
//       application/json:
//         schema:
//           $ref: '#/components/schemas/Error'
// schemas:
//   Error:
//     type: object
//     properties:
//       status:
//         type: string
//       error:
//         type: string
type ErrorResponse struct {
 Status string `json:"status"`
 Error  string `json:"error"`
}

// @openapi
// paths:
//
// /:
//   get:
//     tags:
//       - hello
//     summary: Say Hello
//     description: Returns a hello message
//     operationId: sayHello
//     responses:
//       '200':
//         description: successful operation
//         content:
//           application/json:
//             schema:
//               $ref: '#/components/schemas/Message'
func SayHello(w http.ResponseWriter, r *http.Request) {
 msg := Message{
  Content: "Hello world!",
 }
 render.JSON(w, r, msg)
}

func main() {
 apiDefinition := openapigodoc.OpenAPIDefinition{
  OpenAPI: "3.0.0",
  Info: openapigodoc.Info{
   Title:       "Hello API",
   Version:     "1.0.0",
   Description: "A hello world API",
  },
  Servers: []openapigodoc.Server{{URL: "http://localhost:8080"}},
  Tags: []openapigodoc.Tag{
   {
    Name:        "hello",
    Description: "hello related apis",
   },
  },
  ExternalDocs: openapigodoc.ExternalDocs{Description: "Find out more", URL: "http://example.com"},
 }

 openApiDoc, err := openapigodoc.GenerateOpenApiDoc(apiDefinition)
 if err != nil {
  log.Panicf("Logging err: %s\n", err.Error())
 }

 r := chi.NewRouter()
 r.Use(
  render.SetContentType(render.ContentTypeJSON),
  middleware.RequestID,
  middleware.Logger,
  middleware.Recoverer,
  cors.Handler(cors.Options{
   AllowedOrigins:   []string{"https://*", "http://*"},
   AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
   AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
   ExposedHeaders:   []string{"Link"},
   AllowCredentials: false,
   MaxAge:           300, // Maximum value not ignored by any of major browsers
  }),
 )

 r.Get("/", SayHello)
 r.Get("/api-docs", func(w http.ResponseWriter, r *http.Request) {
  w.Write(openApiDoc)
 })
 http.ListenAndServe(":8080", r)
}
```

 Additional API information can also be added directly to the `openapigodoc.OpenAPIDefinition` object's `Components` field. This is useful when you want to add general API definitions that don't directly relate to any structures or functions in the code base.

```go
func main() {
 securitySchemasData := []byte(`
 {
  "petstore_auth": {
   "type": "oauth2",
   "flows": {
    "implicit": {
     "authorizationUrl": "https://petstore3.swagger.io/oauth/authorize",
     "scopes": {
      "write:pets": "modify pets in your account",
      "read:pets": "read your pets"
     }
    }
   }
  },
  "api_key": {
   "type": "apiKey",
   "name": "api_key",
   "in": "header"
  }
 }`)

 var securitySchemas map[string]interface{}
 err := json.Unmarshal(securitySchemasData, &securitySchemas)
 if err != nil {
  log.Panicf("Logging err: %s\n", err.Error()) // panic if there is an error
 }

 apiDefinition := openapigodoc.OpenAPIDefinition{
        ...
  Components: openapigodoc.Components{
   SecuritySchemes: securitySchemas,
  },
 }

 openApiDoc, err := openapigodoc.GenerateOpenApiDoc(apiDefinition)
 if err != nil {
  log.Panicf("Logging err: %s\n", err.Error())
 }
    
    ...
}
```

## Contributing

Please see [CONTRIBUTING](https://github.com/tink3rlabs/openapi-godoc/blob/main/CONTRIBUTING.md). Thank you, contributors!

## License

Released under the [MIT License](https://github.com/tink3rlabs/openapi-godoc/blob/main/LICENSE)

## Credits

This package was inspired by the excellent [swagger-jsdoc](https://github.com/Surnet/swagger-jsdoc) library.
