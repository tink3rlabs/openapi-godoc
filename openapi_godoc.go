// This package generates an [OpenAPI] document from comments in your code annotated with the @openapi keyword.
// It currently supports only the openapi 3.x schema and not the older swagger 2 schema.
//
// [OpenAPI]: https://swagger.io/specification/v3/
package openapigodoc

import (
	"encoding/json"
	"fmt"
	"go/doc"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/TwiN/deepmerge"
	"github.com/getkin/kin-openapi/openapi3"
	"sigs.k8s.io/yaml"
)

// Info is specified by OpenAPI standard version 3. See https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.3.md#info-object
type Info openapi3.Info

// Security https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.3.md#security-scheme-object
type Security openapi3.SecurityRequirements

// Server is specified by OpenAPI/Swagger standard version 3. See https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.3.md#server-object
type Server openapi3.Server

// Tag is specified by OpenAPI/Swagger 3.0 standard. See https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.3.md#tag-object
type Tag openapi3.Tag

// ExternalDocs is specified by OpenAPI/Swagger standard version 3. See https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.3.md#external-documentation-object
type ExternalDocs openapi3.ExternalDocs

// Components is used to add additional static definitions to the OpenAPIDefinition object
// which are then combined with structs and funcs decorated with @openapi comments
type Components struct {
	Schemas         map[string]interface{} `json:"schemas,omitempty" yaml:"schemas,omitempty"`
	Parameters      map[string]interface{} `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	SecuritySchemes map[string]interface{} `json:"securitySchemes,omitempty" yaml:"securitySchemes,omitempty"`
	RequestBodies   map[string]interface{} `json:"requestBodies,omitempty" yaml:"requestBodies,omitempty"`
	Responses       map[string]interface{} `json:"responses,omitempty" yaml:"responses,omitempty"`
	Headers         map[string]interface{} `json:"headers,omitempty" yaml:"headers,omitempty"`
	Examples        map[string]interface{} `json:"examples,omitempty" yaml:"examples,omitempty"`
	Links           map[string]interface{} `json:"links,omitempty" yaml:"links,omitempty"`
	Callbacks       map[string]interface{} `json:"callbacks,omitempty" yaml:"callbacks,omitempty"`
}

// OpenAPIDefinition is used to define general properties of an API which is then combined with other definitions to generate an OpenAPI document
type OpenAPIDefinition struct {
	OpenAPI      string       `json:"openapi" yaml:"openapi"`
	Info         Info         `json:"info" yaml:"info"`
	Security     Security     `json:"security,omitempty" yaml:"security,omitempty"`
	Servers      []Server     `json:"servers,omitempty" yaml:"servers,omitempty"`
	Tags         []Tag        `json:"tags,omitempty" yaml:"tags,omitempty"`
	ExternalDocs ExternalDocs `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`
	Components   Components   `json:"components,omitempty" yaml:"components,omitempty"`
}

// walk returns an array of directory names to scan when looking for go files with comments
func walk() ([]string, error) {
	dirs := []string{"./"}
	err := filepath.WalkDir("./", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() && (fmt.Sprintf("%c", d.Name()[0]) != ".") {
			dirs = append(dirs, path)
		}
		return nil
	})
	return dirs, err
}

// parseAndMergeComment parses a comment to extract OpenAPI definitions and merges it with an input definition
func parseAndMergeComment(definition []byte, comment string) ([]byte, error) {
	firstWord := strings.Split(comment, "\n")[0]
	if firstWord == "@openapi" {
		def := strings.Replace(comment, firstWord, "", 1)
		def = strings.ReplaceAll(def, "\t", "  ")
		schema, err := yaml.YAMLToJSON([]byte(def))
		if err != nil {
			return nil, fmt.Errorf("failed to convert comment yaml to json: %w", err)
		}
		definition, err = deepmerge.JSON(definition, schema)
		if err != nil {
			return nil, fmt.Errorf("failed to merge parsed comment to OpenAPI defnition: %w", err)
		}
	}
	return definition, nil
}

// generate generates the OpenAPI definition
func generate(definition OpenAPIDefinition, validate bool) ([]byte, error) {
	dirs, err := walk()
	if err != nil {
		return nil, fmt.Errorf("failed to list project directories: %w", err)
	}
	apiDefinition, err := json.Marshal(definition)
	if err != nil {
		return nil, fmt.Errorf("failed marshal definition into an OpenApi document: %w", err)
	}

	for _, path := range dirs {
		fset := token.NewFileSet()
		d, err := parser.ParseDir(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("failed parse documentation at path %s: %w", path, err)
		}

		for _, f := range d {
			p := doc.New(f, "./", 2)

			for _, t := range p.Types {
				apiDefinition, err = parseAndMergeComment(apiDefinition, t.Doc)
				if err != nil {
					return nil, fmt.Errorf("failed to parse OpenApi defnition for struct %s into OpenApi document: %w", t.Name, err)
				}
				for _, m := range t.Methods {
					apiDefinition, err = parseAndMergeComment(apiDefinition, m.Doc)
					if err != nil {
						return nil, fmt.Errorf("failed to parse OpenApi defnition for method %s of struct %s into OpenApi document: %w", m.Name, t.Name, err)
					}
				}
			}

			for _, f := range p.Funcs {
				apiDefinition, err = parseAndMergeComment(apiDefinition, f.Doc)
				if err != nil {
					return nil, fmt.Errorf("failed to parse OpenApi defnition for func %s into OpenApi document: %w", f.Name, err)
				}
			}
		}
	}

	if validate {
		_, err = ValidateOpenApiDoc(apiDefinition)
		if err != nil {
			return nil, fmt.Errorf("failed to validate OpenApi document: %w", err)
		}
	}
	return apiDefinition, nil
}

// ValidateOpenApiDoc validates a document conforms to the OpenAPI 3 specification
func ValidateOpenApiDoc(doc []byte) (bool, error) {
	loader := openapi3.NewLoader()
	parsed, _ := loader.LoadFromData([]byte(doc))
	err := parsed.Validate(loader.Context)
	if err != nil {
		return false, fmt.Errorf("OpenApi document validation failed: %w", err)
	}
	return true, nil
}

// GenerateOpenApiDoc parses all struct and func comments decorated with the @openapi keyword
// as well as any static definitions added directly to the OpenAPIDefinition object and generates an
// OpenAPI document that conforms to the OpenAPI 3 specification
func GenerateOpenApiDoc(definition OpenAPIDefinition, validate bool) ([]byte, error) {
	openApiDoc, err := generate(definition, validate)
	if err != nil {
		return nil, err
	}
	return openApiDoc, nil
}
