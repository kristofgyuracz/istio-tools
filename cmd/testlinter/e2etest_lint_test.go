// Copyright 2018 Istio Authors. All Rights Reserved.
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
	"reflect"
	"testing"

	"github.com/kristofgyuracz/istio-tools/cmd/testlinter/rules"
	"github.com/kristofgyuracz/istio-tools/pkg/checker"
)

func TestE2eTestSkipByIssueRule(t *testing.T) {
	clearLintRulesList()
	LintRulesList[E2eTest] = []checker.Rule{rules.NewSkipByIssue()}

	rpts, _ := getReport([]string{"testdata/"})
	expectedRpts := []string{
		getAbsPath("testdata/e2e/e2e_test.go") + ":26:2:Only t.Skip() is allowed and t.Skip() should contain an url to GitHub issue. (skip_issue)",
	}

	if !reflect.DeepEqual(rpts, expectedRpts) {
		t.Errorf("lint reports don't match\nReceived: %v\nExpected: %v", rpts, expectedRpts)
	}
}

func TestE2eTestSkipByShortRule(t *testing.T) {
	clearLintRulesList()
	LintRulesList[E2eTest] = []checker.Rule{rules.NewSkipByShort()}

	rpts, _ := getReport([]string{"testdata/"})
	expectedRpts := []string{
		getAbsPath("testdata/e2e/e2e_test.go") +
			":25:1:Missing either 'if testing.Short() { t.Skip() }' or 'if !testing.Short() {}' (short_skip)",
		getAbsPath("testdata/e2e/e2e_test.go") +
			":41:1:Missing either 'if testing.Short() { t.Skip() }' or 'if !testing.Short() {}' (short_skip)",
	}

	if !reflect.DeepEqual(rpts, expectedRpts) {
		t.Errorf("lint reports don't match\nReceived: %v\nExpected: %v", rpts, expectedRpts)
	}
}
