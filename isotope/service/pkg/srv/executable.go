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

package srv

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sync"
	"time"

	multierror "github.com/hashicorp/go-multierror"

	"istio.io/pkg/log"
	"github.com/kristofgyuracz/istio-tools/isotope/convert/pkg/graph/script"
	"github.com/kristofgyuracz/istio-tools/isotope/convert/pkg/graph/svctype"
	"github.com/kristofgyuracz/istio-tools/isotope/service/pkg/srv/prometheus"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func execute(
	step interface{},
	forwardableHeader http.Header,
	serviceTypes map[string]svctype.ServiceType) error {
	switch cmd := step.(type) {
	case script.SleepCommand:
		executeSleepCommand(cmd)
	case script.RequestCommand:
		if err := executeRequestCommand(
			cmd, forwardableHeader, serviceTypes); err != nil {
			return err
		}
	case script.ConcurrentCommand:
		if err := executeConcurrentCommand(
			cmd, forwardableHeader, serviceTypes); err != nil {
			return err
		}
	default:
		log.Fatalf("unknown command type in script: %T", cmd)
	}
	return nil
}

func executeSleepCommand(cmd script.SleepCommand) {
	time.Sleep(time.Duration(cmd))
}

func shouldSkipRequest(cmd script.RequestCommand) bool {
	// Probability not set, always send a request
	if cmd.Probability == 0 {
		return false
	}
	return rand.Intn(100) < (100 - cmd.Probability)
}

// Execute sends an HTTP request to another service. Assumes DNS is available
// which maps exe.ServiceName to the relevant URL to reach the service.
func executeRequestCommand(
	cmd script.RequestCommand,
	forwardableHeader http.Header,
	serviceTypes map[string]svctype.ServiceType) error {

	if shouldSkipRequest(cmd) {
		return nil
	}

	destName := cmd.ServiceName
	_, ok := serviceTypes[destName]
	if !ok {
		return fmt.Errorf("service %s does not exist", destName)
	}
	response, err := sendRequest(destName, cmd.Size, forwardableHeader)
	if err != nil {
		return err
	}

	// Necessary for reusing HTTP/1.x "keep-alive" TCP connections.
	// https://golang.org/pkg/net/http/#Response
	defer readAllAndClose(response.Body)
	defer prometheus.RecordRequestSent(destName, uint64(cmd.Size))

	log.Debugf("%s responded with %s", destName, response.Status)
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"service %s responded with %s", destName, response.Status)
	}

	return nil
}

func readAllAndClose(r io.ReadCloser) error {
	if _, err := io.Copy(ioutil.Discard, r); err != nil {
		return err
	}
	return r.Close()
}

// executeConcurrentCommand calls each command in exe.Commands asynchronously
// and waits for each to complete.
func executeConcurrentCommand(
	cmd script.ConcurrentCommand,
	forwardableHeader http.Header,
	serviceTypes map[string]svctype.ServiceType) (errs error) {
	numSubCmds := len(cmd)
	wg := sync.WaitGroup{}
	wg.Add(numSubCmds)
	for _, subCmd := range cmd {
		go func(step interface{}) {
			defer wg.Done()

			err := execute(step, forwardableHeader, serviceTypes)
			if err != nil {
				errs = multierror.Append(errs, err)
			}
		}(subCmd)
	}
	wg.Wait()
	return
}
