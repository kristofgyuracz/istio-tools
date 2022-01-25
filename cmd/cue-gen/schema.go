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
	"strings"

	"cuelang.org/go/encoding/openapi"
)

type SchemaModifier func(kv openapi.KeyValue, om *openapi.OrderedMap, pos int)

func schemaDescriptionModifier(kv openapi.KeyValue, om *openapi.OrderedMap, pos int) {
	if kv.Key != "description" {
		return
	}

	description := strings.ReplaceAll(formatDescription(kv.Value.(string), nil, false), "\n", " ")

	if description == "" {
		om.Elts = append(om.Elts[:pos], om.Elts[pos+1:]...)
	} else {
		om.Set(kv.Key, description)
	}
}

func schemaIntOrStringModifier(kv openapi.KeyValue, om *openapi.OrderedMap, pos int) {
	var typePos int
	params := make(CueGenParameters, 0)
	for k, v := range om.Pairs() {
		if v.Key == "description" {
			_, rawMarkers := parseDescription(v.Value.(string))
			params = parseCueGenParameters(rawMarkers)
		}
		if v.Key == "type" {
			typePos = k
		}
	}

	if p := params.Get("intorstring"); p != nil && p.Value == "true" {
		om.Elts = append(om.Elts[:typePos], om.Elts[typePos+1:]...)
		om.Set("oneOf", []map[string]string{
			{
				"type": "string",
			},
			{
				"type": "integer",
			},
		})
	}
}

func schemaSetterModifier(kv openapi.KeyValue, om *openapi.OrderedMap, pos int) {
	params := make(CueGenParameters, 0)
	for _, v := range om.Pairs() {
		if v.Key == "description" {
			_, rawMarkers := parseDescription(v.Value.(string))
			params = parseCueGenParameters(rawMarkers)
		}
	}

	for _, p := range params.GetAll("set") {
		pieces := strings.SplitN(p.Value, ":", 2)
		if len(pieces) == 2 {
			om.Set(pieces[0], pieces[1])
		}
	}
}
