// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package debuglog provides level-gated diagnostic output, raised with -v.
// Everything goes to stderr, leaving stdout for the YAML.
package debuglog

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// Verbosity levels. Each is a superset of the one below it.
const (
	// LevelQuiet reports only the outcome and per-resource skips.
	LevelQuiet = 0
	// LevelDiagnose adds what a user needs to explain an unexpected result, and
	// permits extra AWS calls to obtain it.
	LevelDiagnose = 1
	// LevelTrace adds a line per request, page and resolution.
	LevelTrace = 2
)

var (
	mu    sync.Mutex
	level int
	out   io.Writer = os.Stderr
)

func SetLevel(l int) {
	mu.Lock()
	defer mu.Unlock()
	level = l
}

// V reports whether the current verbosity is at least l, so a caller can skip work
// that would otherwise be discarded.
func V(l int) bool {
	mu.Lock()
	defer mu.Unlock()
	return level >= l
}

func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	out = w
}

// Logf writes one trace line, emitted at LevelTrace and above.
func Logf(format string, args ...interface{}) { emit(LevelTrace, format, args...) }

// Diagf writes one diagnostic line, emitted at LevelDiagnose and above. Pairing it with
// the V(LevelDiagnose) that guards the work keeps a call from being made and then dropped.
func Diagf(format string, args ...interface{}) { emit(LevelDiagnose, format, args...) }

func emit(l int, format string, args ...interface{}) {
	if !V(l) {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	msg := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Fprint(out, "debug: "+msg)
}

// Section writes a heading that separates phases of a trace.
func Section(name string) {
	Logf("── %s ──", name)
}
