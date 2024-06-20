package openapigodoc

import (
	"testing"
)

// @openapi
// components:
//
//	schemas:
//	  Message:
//	    type: object
//	    properties:
//	      content:
//	        type: string
//	        description: The contents of a message
//	        example: Hello world!
type Message struct {
	Content string `json:"content"`
}

// @openapi
// components:
//
//	responses:
//	  NotFound:
//	    description: The specified resource was not found
//	    content:
//	      application/json:
//	        schema:
//	          $ref: '#/components/schemas/Error'
//	  Unauthorized:
//	    description: Unauthorized
//	    content:
//	      application/json:
//	        schema:
//	          $ref: '#/components/schemas/Error'
//	schemas:
//	  Error:
//	    type: object
//	    properties:
//	      status:
//	        type: string
//	      error:
//	        type: string
type ErrorResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// @openapi
// paths:
//
//	/:
//	  get:
//	    tags:
//	      - hello
//	    summary: Say Hello
//	    description: Returns a hello message
//	    operationId: sayHello
//	    responses:
//	      '200':
//	        description: successful operation
//	        content:
//	          application/json:
//	            schema:
//	              $ref: '#/components/schemas/Message'
func SayHello() {}

var petstore = []byte(`{ "openapi": "3.0.0", "info": { "version": "1.0.0", "title": "Swagger Petstore", "license": { "name": "MIT" } }, "servers": [ { "url": "http://petstore.swagger.io/v1" } ], "paths": { "/pets": { "get": { "summary": "List all pets", "operationId": "listPets", "tags": [ "pets" ], "parameters": [ { "name": "limit", "in": "query", "description": "How many items to return at one time (max 100)", "required": false, "schema": { "type": "integer", "maximum": 100, "format": "int32" } } ], "responses": { "200": { "description": "A paged array of pets", "headers": { "x-next": { "description": "A link to the next page of responses", "schema": { "type": "string" } } }, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Pets" } } } }, "default": { "description": "unexpected error", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Error" } } } } } }, "post": { "summary": "Create a pet", "operationId": "createPets", "tags": [ "pets" ], "requestBody": { "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Pet" } } }, "required": true }, "responses": { "201": { "description": "Null response" }, "default": { "description": "unexpected error", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Error" } } } } } } }, "/pets/{petId}": { "get": { "summary": "Info for a specific pet", "operationId": "showPetById", "tags": [ "pets" ], "parameters": [ { "name": "petId", "in": "path", "required": true, "description": "The id of the pet to retrieve", "schema": { "type": "string" } } ], "responses": { "200": { "description": "Expected response to a valid request", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Pet" } } } }, "default": { "description": "unexpected error", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Error" } } } } } } } }, "components": { "schemas": { "Pet": { "type": "object", "required": [ "id", "name" ], "properties": { "id": { "type": "integer", "format": "int64" }, "name": { "type": "string" }, "tag": { "type": "string" } } }, "Pets": { "type": "array", "maxItems": 100, "items": { "$ref": "#/components/schemas/Pet" } }, "Error": { "type": "object", "required": [ "code", "message" ], "properties": { "code": { "type": "integer", "format": "int32" }, "message": { "type": "string" } } } } } }`)

func TestValidateError(t *testing.T) {
	expectedResult := false
	expectedErr := "OpenApi document validation failed: value of openapi must be a non-empty string"
	actualResult, actualErr := ValidateOpenApiDoc([]byte{})

	if actualErr.Error() != expectedErr {
		t.Errorf("Error actual = %v, and Expected = %v.", actualErr, expectedErr)
	}
	if actualResult != expectedResult {
		t.Errorf("Error actual = %v, and Expected = %v.", actualResult, expectedResult)
	}
}

func TestValidate(t *testing.T) {
	expectedResult := true
	actualResult, actualErr := ValidateOpenApiDoc(petstore)

	if actualErr != nil {
		t.Errorf("Error actual = %v, and Expected = %v.", actualErr, nil)
	}
	if actualResult != expectedResult {
		t.Errorf("Error actual = %v, and Expected = %v.", actualResult, expectedResult)
	}
}

func TestGenerateError(t *testing.T) {
	def := OpenAPIDefinition{}
	expectedResult := []byte{}
	expectedErr := "failed to validate OpenApi document: OpenApi document validation failed: value of openapi must be a non-empty string"
	actualResult, actualErr := GenerateOpenApiDoc(def, true)

	if actualErr.Error() != expectedErr {
		t.Errorf("Error actual = %v, and Expected = %v.", actualErr, expectedErr)
	}
	if actualResult != nil {
		t.Errorf("Error actual = %v, and Expected = %v.", actualResult, expectedResult)
	}
}

func TestGenerate(t *testing.T) {
	def := OpenAPIDefinition{
		OpenAPI: "3.0.0",
		Info: Info{
			Title:       "Hello API",
			Version:     "1.0.0",
			Description: "A hello world API",
		},
		Servers: []Server{{URL: "http://localhost:8080"}},
		Tags: []Tag{
			{
				Name:        "hello",
				Description: "hello related apis",
			},
		},
		ExternalDocs: ExternalDocs{Description: "Find out more", URL: "http://example.com"},
	}
	expectedResult := []byte(`{"components":{"responses":{"NotFound":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Error"}}},"description":"The specified resource was not found"},"Unauthorized":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Error"}}},"description":"Unauthorized"}},"schemas":{"Error":{"properties":{"error":{"type":"string"},"status":{"type":"string"}},"type":"object"},"Message":{"properties":{"content":{"description":"The contents of a message","example":"Hello world!","type":"string"}},"type":"object"}}},"externalDocs":{"description":"Find out more","url":"http://example.com"},"info":{"description":"A hello world API","title":"Hello API","version":"1.0.0"},"openapi":"3.0.0","paths":{"/":{"get":{"description":"Returns a hello message","operationId":"sayHello","responses":{"200":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Message"}}},"description":"successful operation"}},"summary":"Say Hello","tags":["hello"]}}},"servers":[{"url":"http://localhost:8080"}],"tags":[{"description":"hello related apis","name":"hello"}]}`)
	actualResult, actualErr := GenerateOpenApiDoc(def, true)

	if actualErr != nil {
		t.Errorf("Error actual = %v, and Expected = %v.", actualErr, nil)
	}
	if string(actualResult) != string(expectedResult) {
		t.Errorf("Error actual = %v, and Expected = %v.", actualResult, expectedResult)
	}
}
