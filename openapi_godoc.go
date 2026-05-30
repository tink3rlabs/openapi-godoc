// This package generates an [OpenAPI] document from comments in your code annotated with the @openapi keyword.
// It currently supports only the openapi 3.x schema and not the older swagger 2 schema.
//
// [OpenAPI]: https://swagger.io/specification/v3/
package openapigodoc

import (
	"encoding/json"
	"fmt"
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

// openapiBlock is a single @openapi YAML body plus a position string for
// error context. The position is the file:line of the comment group the
// block was extracted from.
type openapiBlock struct {
	pos  string
	body string
}

// openapiBlocks parses every Go file in a single directory (non-recursively)
// and returns every @openapi block found in any comment group — godoc on
// declarations, free-standing file-level comments, and inline comments alike.
// A block starts at a comment line whose trimmed text equals "@openapi" and
// extends until the next such line in the same comment group, or end of group.
//
// Walking f.Comments directly (rather than doc.Package's Types/Methods/Funcs
// view) lifts two artificial constraints: godoc and @openapi can coexist on
// the same declaration, and an operation can be declared without a host
// func — useful when multiple routes (e.g. nested router mounts, aliases,
// versioned mirrors) share a single handler.
func openapiBlocks(path string) ([]openapiBlock, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	var blocks []openapiBlock
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(path, entry.Name()), nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		for _, group := range f.Comments {
			groupPos := fset.Position(group.Pos())
			for _, eb := range extractBlocks(group.Text()) {
				blocks = append(blocks, openapiBlock{
					pos:  fmt.Sprintf("%s:%d", groupPos.Filename, groupPos.Line+eb.lineOffset),
					body: eb.body,
				})
			}
		}
	}
	return blocks, nil
}

// extractedBlock is the raw output of scanning one comment group's text: the
// YAML body plus the line offset of the @openapi marker within the scanned
// text. openapiBlocks turns the offset into an absolute file:line so
// YAML/merge errors locate the right block in a multi-block group.
type extractedBlock struct {
	lineOffset int
	body       string
}

// extractBlocks splits a comment group's text into @openapi YAML bodies.
// A block starts at any line whose trimmed text equals "@openapi" and runs
// until the next such marker in the same text or end of text. The marker
// line is discarded; YAML indentation in body lines is preserved.
//
// Operates on *ast.CommentGroup.Text() output (comment markers stripped,
// blank lines normalized). lineOffset is therefore an approximate source
// line — leading/interior blank comment lines that Text() collapses can
// shift it slightly earlier than the literal marker line. The file and
// group are exact; the offset is good enough to scroll to. Pure and
// side-effect-free.
func extractBlocks(text string) []extractedBlock {
	lines := strings.Split(text, "\n")
	var out []extractedBlock
	i := 0
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) != "@openapi" {
			i++
			continue
		}
		marker := i
		i++ // skip marker line
		start := i
		for i < len(lines) && strings.TrimSpace(lines[i]) != "@openapi" {
			i++
		}
		out = append(out, extractedBlock{
			lineOffset: marker,
			body:       strings.Join(lines[start:i], "\n"),
		})
	}
	return out
}

// parseAndMergeComment converts one @openapi YAML body to JSON and deep-merges
// it into the running OpenAPI definition.
func parseAndMergeComment(definition []byte, block openapiBlock) ([]byte, error) {
	body := strings.ReplaceAll(block.body, "\t", "  ")
	schema, err := yaml.YAMLToJSON([]byte(body))
	if err != nil {
		return nil, fmt.Errorf("failed to convert comment yaml to json: %w", err)
	}
	definition, err = deepmerge.JSON(definition, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to merge parsed comment to OpenAPI definition: %w", err)
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
		blocks, err := openapiBlocks(path)
		if err != nil {
			return nil, fmt.Errorf("failed parse documentation at path %s: %w", path, err)
		}

		for _, b := range blocks {
			apiDefinition, err = parseAndMergeComment(apiDefinition, b)
			if err != nil {
				return nil, fmt.Errorf("failed to parse OpenApi definition at %s into OpenApi document: %w", b.pos, err)
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
