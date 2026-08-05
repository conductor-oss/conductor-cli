/*
 * Copyright 2026 Conductor Authors.
 * <p>
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 * <p>
 * http://www.apache.org/licenses/LICENSE-2.0
 * <p>
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package taskworker

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dop251/goja"
	log "github.com/sirupsen/logrus"
)

// gojaResult is the JSON contract a JavaScript worker returns from its script.
type gojaResult struct {
	Status string                 `json:"status"`
	Body   map[string]interface{} `json:"body"`
}

// GojaHandler runs a JavaScript worker in the CLI's embedded interpreter.
//
// The program is compiled once and each task gets a fresh goja.Runtime: Runtimes are not
// safe for concurrent use, and a Handler is shared across the goroutines of a batch poll.
type GojaHandler struct {
	program *goja.Program
}

// NewGojaHandler compiles script for repeated execution. name appears in stack traces.
func NewGojaHandler(script, name string) (*GojaHandler, error) {
	program, err := goja.Compile(name, script, false)
	if err != nil {
		return nil, fmt.Errorf("error compiling JavaScript worker: %w", err)
	}
	return &GojaHandler{program: program}, nil
}

func (h *GojaHandler) Handle(ctx context.Context, t Task) Result {
	log.Infof("Processing task: %s (workflow: %s)", t.ID, t.WorkflowID)

	vm := goja.New()

	var taskObj interface{}
	if err := json.Unmarshal(t.Raw, &taskObj); err != nil {
		log.Errorf("Error unmarshaling task: %v", err)
		return gojaFailure(fmt.Sprintf("Error unmarshaling task: %v", err))
	}

	dollarObj := vm.NewObject()
	if err := dollarObj.Set("task", taskObj); err != nil {
		log.Errorf("Error setting task in $: %v", err)
		return gojaFailure(fmt.Sprintf("Error setting task: %v", err))
	}
	if err := vm.Set("$", dollarObj); err != nil {
		log.Errorf("Error setting $ object: %v", err)
		return gojaFailure(fmt.Sprintf("Error setting $ object: %v", err))
	}

	injectUtilities(vm)

	value, err := vm.RunProgram(h.program)
	if err != nil {
		log.Errorf("Error executing script for task %s: %v", t.ID, err)
		return gojaFailure(fmt.Sprintf("Script execution error: %v", err))
	}

	return gojaResultToResult(value)
}

// gojaFailure builds the failure shape JavaScript workers have always produced: the
// message lands under an "error" output key rather than in ReasonForIncompletion, which
// workflows may read as ${task.output.error}.
func gojaFailure(reason string) Result {
	return Result{Status: StatusFailed, Output: map[string]interface{}{"error": reason}}
}

// gojaResultToResult interprets whatever the script returned.
//
// A script may return {status, body}, or any other value, or nothing at all; each case
// has an established meaning that workflows depend on. Note that the status is passed
// through verbatim — unlike stdio workers, JavaScript workers may return statuses the
// CLI does not model, such as FAILED_WITH_TERMINAL_ERROR.
func gojaResultToResult(value goja.Value) Result {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return Result{Status: StatusCompleted, Output: map[string]interface{}{}}
	}

	exported := value.Export()
	resultBytes, err := json.Marshal(exported)
	if err != nil {
		log.Errorf("Error marshaling script result: %v", err)
		return Result{Status: StatusCompleted, Output: map[string]interface{}{}}
	}

	var parsed gojaResult
	if err := json.Unmarshal(resultBytes, &parsed); err != nil || parsed.Status == "" {
		log.Warnf("Script result not in expected format, treating as completed")
		return Result{Status: StatusCompleted, Output: map[string]interface{}{"result": exported}}
	}

	body := parsed.Body
	if body == nil {
		body = make(map[string]interface{})
	}
	return Result{Status: Status(parsed.Status), Output: body}
}

func injectUtilities(vm *goja.Runtime) {
	// HTTP utilities
	httpObj := vm.NewObject()
	httpObj.Set("get", func(url string, headers map[string]interface{}) map[string]interface{} {
		return httpRequest("GET", url, headers, "")
	})
	httpObj.Set("post", func(url string, headers map[string]interface{}, body string) map[string]interface{} {
		return httpRequest("POST", url, headers, body)
	})
	httpObj.Set("put", func(url string, headers map[string]interface{}, body string) map[string]interface{} {
		return httpRequest("PUT", url, headers, body)
	})
	httpObj.Set("delete", func(url string, headers map[string]interface{}) map[string]interface{} {
		return httpRequest("DELETE", url, headers, "")
	})
	vm.Set("http", httpObj)

	// Crypto utilities
	cryptoObj := vm.NewObject()
	cryptoObj.Set("md5", func(text string) string {
		hash := md5.Sum([]byte(text))
		return hex.EncodeToString(hash[:])
	})
	cryptoObj.Set("sha1", func(text string) string {
		hash := sha1.Sum([]byte(text))
		return hex.EncodeToString(hash[:])
	})
	cryptoObj.Set("sha256", func(text string) string {
		hash := sha256.Sum256([]byte(text))
		return hex.EncodeToString(hash[:])
	})
	cryptoObj.Set("base64Encode", func(text string) string {
		return base64.StdEncoding.EncodeToString([]byte(text))
	})
	cryptoObj.Set("base64Decode", func(text string) string {
		decoded, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			return ""
		}
		return string(decoded)
	})
	vm.Set("crypto", cryptoObj)

	// Utility functions
	utilObj := vm.NewObject()
	utilObj.Set("sleep", func(ms int) {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	})
	utilObj.Set("uuid", func() string {
		return fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
	})
	utilObj.Set("env", func(key string) string {
		return os.Getenv(key)
	})
	vm.Set("util", utilObj)

	// String utilities
	stringObj := vm.NewObject()
	stringObj.Set("toUpper", strings.ToUpper)
	stringObj.Set("toLower", strings.ToLower)
	stringObj.Set("trim", strings.TrimSpace)
	stringObj.Set("split", func(s, sep string) []string {
		return strings.Split(s, sep)
	})
	stringObj.Set("join", func(arr []string, sep string) string {
		return strings.Join(arr, sep)
	})
	stringObj.Set("replace", func(s, old, new string) string {
		return strings.ReplaceAll(s, old, new)
	})
	stringObj.Set("contains", strings.Contains)
	stringObj.Set("hasPrefix", strings.HasPrefix)
	stringObj.Set("hasSuffix", strings.HasSuffix)
	vm.Set("str", stringObj)
}

func httpRequest(method, url string, headers map[string]interface{}, body string) map[string]interface{} {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return map[string]interface{}{
			"error":  err.Error(),
			"status": 0,
		}
	}

	for key, value := range headers {
		if strVal, ok := value.(string); ok {
			req.Header.Set(key, strVal)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{
			"error":  err.Error(),
			"status": 0,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]interface{}{
			"error":  err.Error(),
			"status": resp.StatusCode,
		}
	}

	var jsonBody interface{}
	if err := json.Unmarshal(respBody, &jsonBody); err == nil {
		return map[string]interface{}{
			"status": resp.StatusCode,
			"body":   jsonBody,
			"text":   string(respBody),
		}
	}

	return map[string]interface{}{
		"status": resp.StatusCode,
		"text":   string(respBody),
	}
}
