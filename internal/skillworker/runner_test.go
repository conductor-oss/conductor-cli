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

package skillworker

import (
	"encoding/json"
	"testing"

	"github.com/conductor-sdk/conductor-go/sdk/model"
)

// TestTaskFromModelMarshalsInput verifies the bridge maps the SDK task's InputData
// map to json.RawMessage at the seam (no map crosses into the loop/handlers).
func TestTaskFromModelMarshalsInput(t *testing.T) {
	mt := model.Task{
		TaskId:             "t1",
		WorkflowInstanceId: "w1",
		InputData:          map[string]interface{}{"path": "notes.md"},
	}
	task, ok, err := taskFromModel(mt)
	if err != nil || !ok {
		t.Fatalf("taskFromModel: ok=%v err=%v", ok, err)
	}
	if task.ID != "t1" || task.WorkflowID != "w1" {
		t.Errorf("task ids = %+v", task)
	}
	var decoded map[string]string
	if err := json.Unmarshal(task.Input, &decoded); err != nil {
		t.Fatalf("input not valid JSON: %v", err)
	}
	if decoded["path"] != "notes.md" {
		t.Errorf("input = %s", task.Input)
	}
}

// TestWrapResultWrapsUnderResultKey checks the output wrapping the skill agent
// expects, for both an object and a bare-string handler output.
func TestWrapResultWrapsUnderResultKey(t *testing.T) {
	obj := wrapResult(json.RawMessage(`{"files":["a","b"]}`))
	inner, ok := obj[outputKeyResult].(map[string]interface{})
	if !ok {
		t.Fatalf("result not an object: %#v", obj)
	}
	if _, ok := inner["files"]; !ok {
		t.Errorf("wrapped object lost its fields: %#v", inner)
	}

	str := wrapResult(json.RawMessage(`"hello"`))
	if str[outputKeyResult] != "hello" {
		t.Errorf("string result = %#v", str[outputKeyResult])
	}

	// Empty output still produces the result key (nil value).
	empty := wrapResult(nil)
	if _, ok := empty[outputKeyResult]; !ok {
		t.Errorf("empty output missing result key: %#v", empty)
	}
}
