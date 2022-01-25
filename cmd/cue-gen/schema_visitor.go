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
	"sort"
	"strings"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/pointer"
	crdutil "sigs.k8s.io/controller-tools/pkg/crd"
	crdMarkers "sigs.k8s.io/controller-tools/pkg/crd/markers"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

const (
	kubebuilderMarkerTag     = "+kubebuilder"
	cueGenParameterMarkerTag = "+cue-gen-param"
)

var (
	_ crdutil.SchemaVisitor = &preserveUnknownFieldVisitor{}
	_ crdutil.SchemaVisitor = &formatDescriptionVisitor{}
	_ crdutil.SchemaVisitor = &applyKubebuilderMarkersVisitor{}
	_ crdutil.SchemaVisitor = &intOrStringVisitor{}
	_ crdutil.SchemaVisitor = &setRequiredFieldsVisitor{}
	_ crdutil.SchemaVisitor = &inlinePropertiesVisitor{}
)

// a visitor to format field description to a schema
type formatDescriptionVisitor struct {
	maxDescriptionLength *int
}

func (v *formatDescriptionVisitor) Visit(schema *apiextv1.JSONSchemaProps) crdutil.SchemaVisitor {
	if schema == nil {
		return v
	}

	schema.Description = formatDescription(schema.Description, v.maxDescriptionLength, true)

	return v
}

// a visitor to set required fields to a schema
type setRequiredFieldsVisitor struct{}

func (v *setRequiredFieldsVisitor) Visit(schema *apiextv1.JSONSchemaProps) crdutil.SchemaVisitor {
	if schema == nil {
		return v
	}

	requiredFields := make([]string, 0)

	for k, s := range schema.Properties {
		rawMarkers := make([]string, 0)
		_, rawMarkers = parseDescription(s.Description)
		params := parseCueGenParameters(rawMarkers)
		if params := params.GetAll(ProtoAttributeParameter); len(params) > 0 {
			attrs := getProtoAttributes(params)
			if v, ok := attrs["google.api.field_behavior"]; ok && v == "REQUIRED" {
				requiredFields = append(requiredFields, k)
			}
		}
	}

	schema.Required = append(schema.Required, requiredFields...)
	sort.Strings(schema.Required)

	return v
}

// a visitor to mutate intOrString type properties to a schema
type intOrStringVisitor struct{}

func (v *intOrStringVisitor) Visit(schema *apiextv1.JSONSchemaProps) crdutil.SchemaVisitor {
	if schema == nil {
		return v
	}

	var rawMarkers []string
	_, rawMarkers = parseDescription(schema.Description)

	params := parseCueGenParameters(rawMarkers)

	isIntOrString := false
	isIntOrStringMap := false
	var pattern string

	if params := params.GetAll(ProtoAttributeParameter); len(params) > 0 {
		switch strings.Trim(getProtoAttributes(params)["options.intorstring"], "\"") {
		case "true":
			isIntOrString = true
		case "map":
			isIntOrStringMap = true
		}
		switch strings.Trim(getProtoAttributes(params)["intorstring"], "\"") {
		case "true":
			isIntOrString = true
		case "map":
			isIntOrStringMap = true
		}

		switch getProtoAttributes(params)["type"] {
		case "k8s.io.apimachinery.pkg.util.intstr.IntOrString":
			isIntOrString = true
		case "k8s.io.apimachinery.pkg.api.resource.Quantity":
			isIntOrString = true
			pattern = "^(\\+|-)?(([0-9]+(\\.[0-9]*)?)|(\\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\\+|-)?(([0-9]+(\\.[0-9]*)?)|(\\.[0-9]+))))?$"
		case "map<string,k8s.io.apimachinery.pkg.api.resource.Quantity>":
			isIntOrStringMap = true
			pattern = "^(\\+|-)?(([0-9]+(\\.[0-9]*)?)|(\\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\\+|-)?(([0-9]+(\\.[0-9]*)?)|(\\.[0-9]+))))?$"
		}
	}

	modSchema := func(schema *apiextv1.JSONSchemaProps) {
		schema.Properties = nil
		schema.Type = ""
		schema.XIntOrString = true
		schema.AnyOf = []apiextv1.JSONSchemaProps{
			{Type: "integer"},
			{Type: "string"},
		}
		if schema.Pattern == "" && pattern != "" {
			schema.Pattern = pattern
		}
	}

	if isIntOrString {
		modSchema(schema)
	} else if isIntOrStringMap {
		modSchema(schema.AdditionalProperties.Schema)
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

// a visitor to move inline properties
type inlinePropertiesVisitor struct {
	found bool
}

func (v *inlinePropertiesVisitor) Visit(schema *apiextv1.JSONSchemaProps) crdutil.SchemaVisitor {
	if schema == nil {
		return v
	}

	if len(schema.Properties) > 0 {
		props := make(map[string]apiextv1.JSONSchemaProps)
		for k, s := range schema.Properties {
			if v.isInline(s) {
				v.found = true
				for k1, v1 := range s.Properties {
					props[k1] = v1
				}
				continue
			}
			props[k] = s
		}

		schema.Properties = props
	}

	return v
}

func (v *inlinePropertiesVisitor) IsInlineFound() bool {
	return v.found
}

func (v *inlinePropertiesVisitor) Reset() {
	v.found = false
}

func (v *inlinePropertiesVisitor) isInline(schema apiextv1.JSONSchemaProps) bool {
	_, rawMarkers := parseDescription(schema.Description)
	params := parseCueGenParameters(rawMarkers)
	if params := params.GetAll(ProtoAttributeParameter); len(params) > 0 {
		attrs := getProtoAttributes(params)
		if val, ok := attrs["gogoproto.jsontag"]; ok && val == "\",inline\"" {
			return true
		}
	}

	return false
}

func formatDescription(description string, maxLength *int, firstParagraphOnly bool) string {
	var rawMarkers []string
	description, rawMarkers = parseDescription(description)

	if strings.HasPrefix(description, "$hide_from_docs") {
		return ""
	}

	params := parseCueGenParameters(rawMarkers)

	if param := params.Get(IstioPackageNameParameter); param != nil {
		if res, ok := frontMatterMap[param.Value]; ok {
			description = res[0] + " See more details at: " + res[1]
			return description
		}
	}

	if paras := strings.Split(description, ". "); firstParagraphOnly && len(paras) > 0 && paras[0] != "" {
		description = paras[0]

		lines := strings.Split(paras[0], "\n")
		if len(lines) > 0 {
			descLines := []string{}
			for _, line := range lines {
				descLines = append(descLines, line)
				if line[len(line)-1] == '.' {
					break
				}
			}
			description = strings.Join(descLines, "\n")
		}

		if description[len(description)-1] != '.' {
			description = description + "."
		}
	}

	if maxLength != nil && len(description) > *maxLength {
		description = description[0:*maxLength]
	}

	return description
}
