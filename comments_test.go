package openapigodoc

import (
	"strings"
	"testing"
)

// NewPet creates a pet. This is a regular godoc line that coexists with an
// @openapi block on the same declaration. Before this change the parser
// keyed off the comment's first line, so a prefatory sentence like this
// would silently disable detection.
//
// @openapi
// paths:
//
//	/pets:
//	  post:
//	    operationId: createPet
//	    responses:
//	      '201':
//	        description: created
func NewPet() {}

// @openapi
// paths:
//
//	/standalone:
//	  get:
//	    operationId: standaloneOp
//	    responses:
//	      '200':
//	        description: ok

// _ keeps the @openapi block above as its own free-standing comment
// group, unattached to any named decl.
const _ = 0

// MultiBlockHost hosts two @openapi blocks in a single comment group.
// The second block is reached by line-start scanning, not by being the
// first line of the comment.
//
// @openapi
// paths:
//
//	/multi1:
//	  get:
//	    operationId: multi1Op
//	    responses:
//	      '200':
//	        description: ok
//
// @openapi
// paths:
//
//	/multi2:
//	  get:
//	    operationId: multi2Op
//	    responses:
//	      '200':
//	        description: ok
func MultiBlockHost() {}

// PlainGodoc has no @openapi marker and must not appear in the generated
// document. This locks in the "non-marker comment groups are ignored" path
// against future regressions in extractBlocks.
//
// Also exercises the false-positive guard: the literal string @openapi
// appears mid-line below but is not at line-start-trimmed, so it must not
// be picked up as a marker.
//
// Reference: not @openapi unless on its own line.
func PlainGodoc() {}

// TrailingMarkerHost ends with a bare @openapi marker and no body. The
// parser must treat this as a zero-body block (no-op merge), not crash.
//
// @openapi
func TrailingMarkerHost() {}

func TestExtractBlocks(t *testing.T) {
	cases := []struct {
		name      string
		text      string
		wantCount int
		wantBody0 string // body of first block, when wantCount > 0
	}{
		{"no marker", "PlainDoc does X.\n", 0, ""},
		{"single block", "@openapi\npaths:\n  /a: {}\n", 1, "paths:\n  /a: {}\n"},
		{"godoc preceding", "MyFn does X.\n\n@openapi\npaths:\n  /a: {}\n", 1, "paths:\n  /a: {}\n"},
		{"two blocks", "@openapi\npaths:\n  /a: {}\n@openapi\npaths:\n  /b: {}\n", 2, "paths:\n  /a: {}"},
		{"trailing marker empty body", "@openapi\n", 1, ""},
		{"back-to-back markers", "@openapi\n@openapi\nfoo: bar\n", 2, ""},
		{"mid-line @openapi not a marker", "see @openapi for details\n", 0, ""},
		{"CRLF line endings", "@openapi\r\npaths:\r\n  /a: {}\r\n", 1, "paths:\r\n  /a: {}\r\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractBlocks(tc.text)
			if len(got) != tc.wantCount {
				t.Fatalf("got %d blocks, want %d: %+v", len(got), tc.wantCount, got)
			}
			if tc.wantCount > 0 && got[0].body != tc.wantBody0 {
				t.Errorf("first body = %q, want %q", got[0].body, tc.wantBody0)
			}
		})
	}
}

// TestGenerateExpandedSources verifies the parser picks up @openapi blocks
// from sources the pre-refactor parser couldn't reach: godoc-prefixed
// blocks on the same declaration, free-standing comment groups, and
// multiple blocks within a single comment group.
func TestGenerateExpandedSources(t *testing.T) {
	def := OpenAPIDefinition{
		OpenAPI: "3.0.0",
		Info:    Info{Title: "t", Version: "1"},
	}
	out, err := GenerateOpenApiDoc(def, false)
	if err != nil {
		t.Fatalf("GenerateOpenApiDoc returned error: %v", err)
	}
	got := string(out)
	for _, want := range []string{
		`"createPet"`,    // godoc + @openapi on same decl (closes #4)
		`"standaloneOp"`, // free-standing comment group
		`"multi1Op"`,     // first of two blocks in one group
		`"multi2Op"`,     // second of two blocks in one group
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %s; full output:\n%s", want, got)
		}
	}
	// Non-marker comment groups (PlainGodoc) and trailing-marker-only groups
	// (TrailingMarkerHost) must not leak any operation into the doc.
	for _, unwanted := range []string{"PlainGodoc", "TrailingMarkerHost"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("output should not contain %q", unwanted)
		}
	}
}
