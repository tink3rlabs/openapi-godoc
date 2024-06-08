package openapi_godoc

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

func GenerateOpenApiDoc(definition OpenAPIDefinition) ([]byte, error) {
	dirs, _ := walk()
	apiDefinition, err := json.Marshal(definition)
	if err != nil {
		return nil, err
	}

	for _, path := range dirs {
		fset := token.NewFileSet()
		d, err := parser.ParseDir(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, err
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
						return nil, err
					}
					apiDefinition, err = deepmerge.JSON(apiDefinition, schema)
					if err != nil {
						return nil, err
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
						return nil, err
					}
					apiDefinition, err = deepmerge.JSON(apiDefinition, schema)
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return apiDefinition, nil
}
