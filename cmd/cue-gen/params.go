// Copyright 2019 Istio Authors
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
	"fmt"
	"strings"
)

type ParameterName string

const (
	FieldNameParameter           ParameterName = "field-name"
	ImportPathParameter          ParameterName = "import-path"
	IstioPackageNameParameter    ParameterName = "istio-package-name"
	PackageNameParameter         ParameterName = "package-name"
	ProtoAttributeParameter      ParameterName = "proto-attribute"
	ReferenceImportPathParameter ParameterName = "reference-import-path"
)

type CueGenParameter struct {
	Name  ParameterName
	Value string
}

type CueGenParameters []CueGenParameter

func (p CueGenParameters) Get(name ParameterName) *CueGenParameter {
	for _, param := range p {
		if param.Name == name {
			return &param
		}
	}

	return nil
}

func (p CueGenParameters) GetAll(name ParameterName) CueGenParameters {
	params := make(CueGenParameters, 0)
	for _, param := range p {
		if param.Name == name {
			params = append(params, param)
		}
	}

	return params
}

func NewCueGenParameterMarker(name ParameterName, value string) string {
	return CueGenParameter{
		Name:  name,
		Value: value,
	}.String()
}

func (p CueGenParameter) String() string {
	return fmt.Sprintf("%s:%s=%s", cueGenParameterMarkerTag, p.Name, p.Value)
}

func parseCueGenParameterMarker(raw string) *CueGenParameter {
	param := &CueGenParameter{}

	p1 := strings.SplitN(raw, ":", 2)
	if len(p1) != 2 {
		return nil
	}

	if p1[0] != cueGenParameterMarkerTag {
		return nil
	}

	p2 := strings.SplitN(p1[1], "=", 2)
	if len(p2) != 2 {
		return nil
	}

	param.Name = ParameterName(p2[0])
	param.Value = p2[1]

	return param
}

func parseCueGenParameters(markers []string) CueGenParameters {
	params := make(CueGenParameters, 0)
	for _, marker := range markers {
		param := parseCueGenParameterMarker(marker)
		if param == nil {
			continue
		}

		params = append(params, *param)
	}

	return params
}

func getProtoAttributes(params CueGenParameters) map[string]string {
	attrs := make(map[string]string)
	for _, param := range params {
		p := strings.SplitN(param.Value, ":", 2)
		if len(p) != 2 {
			continue
		}

		attrs[p[0]] = p[1]
	}

	return attrs
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
		if strings.HasPrefix(l, "+default") {
			l = fmt.Sprintf("%s:%s", kubebuilderMarkerTag, l[1:])
		}
		if l == "+kubebuilder:validation:Required" {
			l = NewCueGenParameterMarker(ProtoAttributeParameter, "google.api.field_behavior:REQUIRED")
		}
		if strings.HasPrefix(l, "+") {
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
