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

type Info openapi3.Info
type Security openapi3.SecurityRequirements
type Server openapi3.Server
type Tag openapi3.Tag
type ExternalDocs openapi3.ExternalDocs

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

type OpenAPIDefinition struct {
	OpenAPI      string       `json:"openapi" yaml:"openapi"`
	Info         Info         `json:"info" yaml:"info"`
	Security     Security     `json:"security,omitempty" yaml:"security,omitempty"`
	Servers      []Server     `json:"servers,omitempty" yaml:"servers,omitempty"`
	Tags         []Tag        `json:"tags,omitempty" yaml:"tags,omitempty"`
	ExternalDocs ExternalDocs `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`
	Components   Components   `json:"components,omitempty" yaml:"components,omitempty"`
}

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

func generate(definition OpenAPIDefinition) ([]byte, error) {
	dirs, err := walk()
	if err != nil {
		return nil, fmt.Errorf("falied to list project directories: %w", err)
	}
	apiDefinition, err := json.Marshal(definition)
	if err != nil {
		return nil, fmt.Errorf("falied marshal definition into an OpenApi document: %w", err)
	}

	for _, path := range dirs {
		fset := token.NewFileSet()
		d, err := parser.ParseDir(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("falied parse documentation at path %s: %w", path, err)
		}

		for _, f := range d {
			p := doc.New(f, "./", 2)

			for _, t := range p.Types {
				firstWord := strings.Split(t.Doc, "\n")[0]
				if firstWord == "@openapi" {
					def := strings.Replace(t.Doc, firstWord, "", 1)
					def = strings.ReplaceAll(def, "\t", "  ")
					schema, err := yaml.YAMLToJSON([]byte(def))
					if err != nil {
						return nil, fmt.Errorf("falied to convert OpenApi defnition yaml to json for struct %s: %w", t.Name, err)
					}
					apiDefinition, err = deepmerge.JSON(apiDefinition, schema)
					if err != nil {
						return nil, fmt.Errorf("falied to merge OpenApi defnition for struct %s into OpenApi document: %w", t.Name, err)
					}
				}
			}

			for _, f := range p.Funcs {
				firstWord := strings.Split(f.Doc, "\n")[0]
				if firstWord == "@openapi" {
					def := strings.Replace(f.Doc, firstWord, "", 1)
					def = strings.ReplaceAll(def, "\t", "  ")
					schema, err := yaml.YAMLToJSON([]byte(def))
					if err != nil {
						return nil, fmt.Errorf("falied to convert OpenApi defnition yaml to json for func %s: %w", f.Name, err)
					}
					apiDefinition, err = deepmerge.JSON(apiDefinition, schema)
					if err != nil {
						return nil, fmt.Errorf("falied to merge OpenApi defnition for func %s into OpenApi document: %w", f.Name, err)
					}
				}
			}
		}
	}

	_, err = ValidateOpenApiDoc(apiDefinition)
	if err != nil {
		return nil, fmt.Errorf("falied to validate OpenApi document: %w", err)
	}
	return apiDefinition, nil
}

func ValidateOpenApiDoc(doc []byte) (bool, error) {
	loader := openapi3.NewLoader()
	parsed, _ := loader.LoadFromData([]byte(doc))
	err := parsed.Validate(loader.Context)
	if err != nil {
		return false, fmt.Errorf("OpenApi document validation failed: %w", err)
	}
	return true, nil
}

func GenerateOpenApiDoc(definition OpenAPIDefinition) ([]byte, error) {
	openApiDoc, err := generate(definition)
	if err != nil {
		return nil, err
	}
	return openApiDoc, nil
}
