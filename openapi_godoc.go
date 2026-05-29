// This package generates an [OpenAPI] document from comments in your code annotated with the @openapi keyword.
// It currently supports only the openapi 3.x schema and not the older swagger 2 schema.
//
// [OpenAPI]: https://swagger.io/specification/v3/
package openapigodoc

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
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

// docComment pairs a declaration's name with its doc comment text.
type docComment struct {
	name string
	text string
}

// docComments parses every Go file in a single directory (non-recursively) and
// returns the doc comments attached to exported types and to every exported
// function or method (regardless of its receiver type's visibility) — the
// declarations that may carry an @openapi annotation. It reads the AST
// directly, replacing the parser.ParseDir/doc.New pair (both deprecated as of
// Go 1.25) while still covering declarations in _test.go files, which
// doc.NewFromFiles would otherwise skip.
func docComments(path string) ([]docComment, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	var comments []docComment
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(path, entry.Name()), nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.IsExported() && d.Doc != nil {
					comments = append(comments, docComment{d.Name.Name, d.Doc.Text()})
				}
			case *ast.GenDecl:
				if d.Tok != token.TYPE {
					continue
				}
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok || !ts.Name.IsExported() {
						continue
					}
					// The doc comment attaches to the spec for grouped
					// declarations and to the GenDecl for single ones.
					group := ts.Doc
					if group == nil {
						group = d.Doc
					}
					if group != nil {
						comments = append(comments, docComment{ts.Name.Name, group.Text()})
					}
				}
			}
		}
	}
	return comments, nil
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
		comments, err := docComments(path)
		if err != nil {
			return nil, fmt.Errorf("failed parse documentation at path %s: %w", path, err)
		}

		for _, c := range comments {
			apiDefinition, err = parseAndMergeComment(apiDefinition, c.text)
			if err != nil {
				return nil, fmt.Errorf("failed to parse OpenApi defnition for %s into OpenApi document: %w", c.name, err)
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
