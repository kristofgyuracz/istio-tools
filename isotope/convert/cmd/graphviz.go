// Copyright 2018 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this currentFile except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"io/ioutil"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"

	"github.com/kristofgyuracz/istio-tools/isotope/convert/pkg/graph"
	"github.com/kristofgyuracz/istio-tools/isotope/convert/pkg/graphviz"
)

// graphvizCmd represents the graphviz command
var graphvizCmd = &cobra.Command{
	Use:   "graphviz [YAML file] [output file]",
	Short: "Convert a .yaml file to a Graphviz DOT language file",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		inFileName := args[0]
		yamlContents, err := ioutil.ReadFile(inFileName)
		exitIfError(err)

		var serviceGraph graph.ServiceGraph
		err = yaml.Unmarshal(yamlContents, &serviceGraph)
		exitIfError(err)

		dotLang, err := graphviz.ServiceGraphToDotLanguage(serviceGraph)
		exitIfError(err)

		outFileName := args[1]
		err = ioutil.WriteFile(outFileName, []byte(dotLang), 0644)
		exitIfError(err)
	},
}

func init() {
	rootCmd.AddCommand(graphvizCmd)
}
