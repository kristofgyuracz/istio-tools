// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"log"
	"strings"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/pointer"
	crdutil "sigs.k8s.io/controller-tools/pkg/crd"
	crdMarkers "sigs.k8s.io/controller-tools/pkg/crd/markers"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

const (
	kubebuilderMarkerTag = "+kubebuilder"
)

var (
	_ crdutil.SchemaVisitor = &preserveUnknownFieldVisitor{}
	_ crdutil.SchemaVisitor = &formatDescriptionVisitor{}
	_ crdutil.SchemaVisitor = &applyKubebuilderMarkersVisitor{}
	_ crdutil.SchemaVisitor = &intOrStringVisitor{}
)

// a visitor to format field description to a schema
type formatDescriptionVisitor struct{}

func (v *formatDescriptionVisitor) Visit(schema *apiextv1.JSONSchemaProps) crdutil.SchemaVisitor {
	if schema == nil {
		return v
	}

	schema.Description, _ = parseDescription(schema.Description)

	if strings.HasPrefix(schema.Description, "$hide_from_docs") {
		schema.Description = ""
	}
	if paras := strings.Split(schema.Description, ". "); len(paras) > 0 && paras[0] != "" {
		schema.Description = paras[0]

		lines := strings.Split(paras[0], "\n")
		if len(lines) > 0 {
			descLines := []string{}
			for _, line := range lines {
				descLines = append(descLines, line)
				if line[len(line)-1] == '.' {
					break
				}
			}
			schema.Description = strings.Join(descLines, "\n")
		}

		if schema.Description[len(schema.Description)-1] != '.' {
			schema.Description = schema.Description + "."
		}
	}

	return v
}

// a visitor to mutate intOrString type properties to a schema
type intOrStringVisitor struct{}

func (v *intOrStringVisitor) Visit(schema *apiextv1.JSONSchemaProps) crdutil.SchemaVisitor {
	if schema == nil {
		return v
	}

	_, intVal := schema.Properties["intVal"]
	_, strVal := schema.Properties["strVal"]

	if intVal && strVal {
		schema.Properties = nil
		schema.Type = ""
		schema.XIntOrString = true
		schema.AnyOf = []apiextv1.JSONSchemaProps{
			{Type: "integer"},
			{Type: "string"},
		}
	}

	return v
}

// a visitor to apply kubebuilder markers based validations to a schema
type applyKubebuilderMarkersVisitor struct{}

func (v *applyKubebuilderMarkersVisitor) Visit(schema *apiextv1.JSONSchemaProps) crdutil.SchemaVisitor {
	if schema == nil {
		return v
	}

	_, rawMarkers := parseDescription(schema.Description)

	reg := &markers.Registry{}
	for _, def := range crdMarkers.AllDefinitions {
		if err := def.Register(reg); err != nil {
			log.Printf("could not register marker: %v", err)
			continue
		}
	}

	collector := &markers.Collector{Registry: reg}

	for _, marker := range rawMarkers {
		def := collector.Lookup(marker, markers.DescribesField)
		if def == nil {
			continue
		}
		markerValue, err := def.Parse(marker)
		if err != nil {
			log.Printf("could not parse marker: %v", err)
			continue
		}
		schemaMarker, isSchemaMarker := markerValue.(crdutil.SchemaMarker)
		if !isSchemaMarker {
			continue
		}
		err = schemaMarker.ApplyToSchema(schema)
		if err != nil {
			log.Printf("could not apply marker: %v", err)
		}
	}

	return v
}

// a visitor to add x-kubernetes-preserve-unknown-field to a schema
type preserveUnknownFieldVisitor struct {
	// path is in the format of a.b.c to indicate a field path in the schema
	// a `[]` indicates an array, a string is a key to a map in the schema
	// e.g. a.[].b.c
	path string
}

func (v *preserveUnknownFieldVisitor) Visit(schema *apiextv1.JSONSchemaProps) crdutil.SchemaVisitor {
	if schema == nil {
		return v
	}
	p := strings.Split(v.path, ".")
	if len(p) == 0 {
		return nil
	}
	if len(p) == 1 {
		if s, ok := schema.Properties[p[0]]; ok {
			s.XPreserveUnknownFields = pointer.BoolPtr(true)
			schema.Properties[p[0]] = s
		}
		return nil
	}
	if len(p) > 1 {
		if p[0] == "[]" && schema.Items == nil {
			return nil
		}
		if p[0] != "[]" && schema.Items != nil {
			return nil
		}
		if _, ok := schema.Properties[p[0]]; p[0] != "[]" && !ok {
			return nil
		}
		return &preserveUnknownFieldVisitor{path: strings.Join(p[1:], ".")}
	}
	return nil
}

func parseDescription(desc string) (string, []string) {
	lines := strings.Split(desc, "\n")
	outLines := []string{}
	rawMarkers := []string{}
	out := true
	for _, l := range lines {
		l = strings.Trim(l, " ")
		if strings.HasPrefix(l, "<!--") {
			out = false
		}
		if strings.HasPrefix(l, enableCRDGenTag) || strings.HasPrefix(l, kubebuilderMarkerTag) {
			rawMarkers = append(rawMarkers, l)
			continue
		}
		if out && l != "" {
			outLines = append(outLines, l)
		}
		if !out && l == "-->" {
			out = true
		}
	}

	return strings.Join(outLines, "\n"), rawMarkers
}
