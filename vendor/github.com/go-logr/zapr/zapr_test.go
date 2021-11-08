/*
Copyright 2020 The Kubernetes Authors.
Copyright 2021 The logr Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package zapr_test

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
)

const fixedTime = 123.456789

func fixedTimeEncoder(_ time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendFloat64(fixedTime)
}

// discard is a replacement for io.Discard, needed for Go 1.14.
type discard struct{}

func (d discard) Write(p []byte) (n int, err error) { return n, nil }

func newZapLogger(lvl zapcore.Level, w zapcore.WriteSyncer) *zap.Logger {
	if w == nil {
		w = zapcore.AddSync(discard{})
	}
	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey:     "msg",
		CallerKey:      "caller",
		TimeKey:        "ts",
		EncodeTime:     fixedTimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	})
	core := zapcore.NewCore(encoder, zapcore.AddSync(w), lvl)
	l := zap.New(core, zap.WithCaller(true))
	return l
}

// TestInfo tests the JSON info format.
func TestInfo(t *testing.T) {
	type testCase struct {
		msg            string
		format         string
		names          []string
		withKeysValues []interface{}
		keysValues     []interface{}
		wrapper        func(logr.Logger, string, ...interface{})
	}
	var testDataInfo = []testCase{
		{
			msg: "simple",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"simple","v":0}
`,
		},
		{
			msg: "WithCallDepth",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"WithCallDepth","v":0}
`,
			wrapper: myInfo,
		},
		{
			msg: "incremental WithCallDepth",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"incremental WithCallDepth","v":0}
`,
			wrapper: myInfoInc,
		},
		{
			msg: "one name",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"one name","v":0}
`,
			names: []string{"me"},
		},
		{
			msg: "many names",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"many names","v":0}
`,
			names: []string{"hello", "world"},
		},
		{
			msg: "key/value pairs",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"key/value pairs","v":0,"ns":"default","podnum":2}
`,
			keysValues: []interface{}{"ns", "default", "podnum", 2},
		},
		{
			msg: "WithValues",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"WithValues","ns":"default","podnum":2,"v":0}
`,
			withKeysValues: []interface{}{"ns", "default", "podnum", 2},
		},
		{
			msg: "empty WithValues",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"empty WithValues","v":0,"ns":"default","podnum":2}
`,
			withKeysValues: []interface{}{},
			keysValues:     []interface{}{"ns", "default", "podnum", 2},
		},
		{
			msg: "mixed",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"mixed","ns":"default","v":0,"podnum":2}
`,
			withKeysValues: []interface{}{"ns", "default"},
			keysValues:     []interface{}{"podnum", 2},
		},
		{
			msg: "invalid WithValues",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"non-string key argument passed to logging, ignoring all later arguments","invalid key":200}
{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"invalid WithValues","ns":"default","podnum":2,"v":0}
`,
			withKeysValues: []interface{}{"ns", "default", "podnum", 2, 200, "replica", "Running", 10},
		},
		{
			msg: "strongly typed Zap field",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"strongly-typed Zap Field passed to logr","zap field":{"Key":"zap-field-attempt","Type":11,"Integer":3,"String":"","Interface":null}}
{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"strongly typed Zap field","v":0,"ns":"default","podnum":2,"zap-field-attempt":3,"Running":10}
`,
			keysValues: []interface{}{"ns", "default", "podnum", 2, zap.Int("zap-field-attempt", 3), "Running", 10},
		},
		{
			msg: "non-string key argument",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"non-string key argument passed to logging, ignoring all later arguments","invalid key":200}
{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"non-string key argument","v":0,"ns":"default","podnum":2}
`,
			keysValues: []interface{}{"ns", "default", "podnum", 2, 200, "replica", "Running", 10},
		},
		{
			msg: "missing value",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"odd number of arguments passed as key-value pairs for logging","ignored key":"no-value"}
{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"missing value","v":0,"ns":"default","podnum":2}
`,
			keysValues: []interface{}{"ns", "default", "podnum", 2, "no-value"},
		},
		{
			msg: "duration value argument",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"duration value argument","v":0,"duration":"5s"}
`,
			keysValues: []interface{}{"duration", time.Duration(5 * time.Second)},
		},
	}

	test := func(t *testing.T, logNumeric *string, enablePanics *bool, allowZapFields *bool, data testCase) {
		var buffer bytes.Buffer
		writer := bufio.NewWriter(&buffer)
		var sampleInfoLogger logr.Logger
		zl := newZapLogger(zapcore.Level(-100), zapcore.AddSync(writer))
		if logNumeric == nil && enablePanics == nil && allowZapFields == nil {
			// No options.
			sampleInfoLogger = zapr.NewLogger(zl)
		} else {
			opts := []zapr.Option{}
			if logNumeric != nil {
				opts = append(opts, zapr.LogInfoLevel(*logNumeric))
			}
			if enablePanics != nil {
				opts = append(opts, zapr.DPanicOnBugs(*enablePanics))
			}
			if allowZapFields != nil {
				opts = append(opts, zapr.AllowZapFields(*allowZapFields))
			}
			sampleInfoLogger = zapr.NewLoggerWithOptions(zl, opts...)
		}
		if data.withKeysValues != nil {
			sampleInfoLogger = sampleInfoLogger.WithValues(data.withKeysValues...)
		}
		for _, name := range data.names {
			sampleInfoLogger = sampleInfoLogger.WithName(name)
		}
		if data.wrapper != nil {
			data.wrapper(sampleInfoLogger, data.msg, data.keysValues...)
		} else {
			sampleInfoLogger.Info(data.msg, data.keysValues...)
		}
		if err := writer.Flush(); err != nil {
			t.Fatalf("unexpected error from Flush: %v", err)
		}
		logStr := buffer.String()

		logStrLines := strings.Split(logStr, "\n")
		var dataFormatLines []string
		noPanics := enablePanics != nil && !*enablePanics
		withZapFields := allowZapFields != nil && *allowZapFields
		for _, line := range strings.Split(data.format, "\n") {
			// Potentially filter out all or some panic
			// message. We can recognize them based on the
			// expected special keys.
			if strings.Contains(line, "invalid key") ||
				strings.Contains(line, "ignored key") {
				if noPanics {
					continue
				}
			} else if strings.Contains(line, "zap field") {
				if noPanics || withZapFields {
					continue
				}
			}
			haveZapField := strings.Index(line, `"zap-field`)
			if haveZapField != -1 && !withZapFields && !strings.Contains(line, "zap field") {
				// When Zap fields are not allowed, output gets truncated at the first Zap field.
				line = line[0:haveZapField-1] + "}"
			}
			dataFormatLines = append(dataFormatLines, line)
		}
		if !assert.Equal(t, len(logStrLines), len(dataFormatLines)) {
			t.Errorf("Info has wrong format: no. of lines in log is incorrect \n expected: %s\n got: %s", dataFormatLines, logStrLines)
		}

		for i := range logStrLines {
			if len(logStrLines[i]) == 0 && len(dataFormatLines[i]) == 0 {
				continue
			}
			var ts float64
			var lineNo int
			format := dataFormatLines[i]
			if logNumeric == nil || *logNumeric == "" {
				format = regexp.MustCompile(`,"v":-?\d`).ReplaceAllString(format, "")
			}
			n, err := fmt.Sscanf(logStrLines[i], format, &ts, &lineNo)
			if n != 2 || err != nil {
				t.Errorf("log format error: %d elements, error %s:\n%s", n, err, logStrLines[i])
			}
			expected := fmt.Sprintf(format, fixedTime, lineNo)
			require.JSONEq(t, expected, logStrLines[i])
		}
	}

	noV := ""
	v := "v"
	for name, logNumeric := range map[string]*string{"default": nil, "disabled": &noV, "v": &v} {
		t.Run(fmt.Sprintf("numeric level %v", name), func(t *testing.T) {
			yes := true
			no := false
			nilYesNo := map[string]*bool{"default": nil, "yes": &yes, "no": &no}
			for name, panicMessages := range nilYesNo {
				t.Run(fmt.Sprintf("panic messages %s", name), func(t *testing.T) {
					for name, allowZapFields := range nilYesNo {
						t.Run(fmt.Sprintf("allow zap fields %s", name), func(t *testing.T) {
							for _, data := range testDataInfo {
								t.Run(data.msg, func(t *testing.T) {
									test(t, logNumeric, panicMessages, allowZapFields, data)
								})
							}
						})
					}
				})
			}
		})
	}
}

// TestEnabled tests whether log messages are enabled.
func TestEnabled(t *testing.T) {
	for i := 0; i < 11; i++ {
		t.Run(fmt.Sprintf("logger level %d", i), func(t *testing.T) {
			var sampleInfoLogger = zapr.NewLogger(newZapLogger(zapcore.Level(0-i), nil))
			// Very high levels are theoretically possible and need special
			// handling because zap uses int8.
			for j := 0; j <= 128; j++ {
				shouldBeEnabled := i >= j
				t.Run(fmt.Sprintf("message level %d", j), func(t *testing.T) {
					isEnabled := sampleInfoLogger.V(j).Enabled()
					if !isEnabled && shouldBeEnabled {
						t.Errorf("V(%d).Info should be enabled", j)
					} else if isEnabled && !shouldBeEnabled {
						t.Errorf("V(%d).Info should not be enabled", j)
					}

					log := sampleInfoLogger
					for k := 0; k < j; k++ {
						log = log.V(1)
					}
					isEnabled = log.Enabled()
					if !isEnabled && shouldBeEnabled {
						t.Errorf("repeated V(1).Info should be enabled")
					} else if isEnabled && !shouldBeEnabled {
						t.Errorf("repeated V(1).Info should not be enabled")
					}
				})
			}
		})
	}
}

// TestV tests support for numeric log level logging.
func TestLogNumeric(t *testing.T) {
	for logNumeric, formatted := range map[string]string{"": "", "v": `"v":%d,`, "verbose": `"verbose":%d,`} {
		t.Run(fmt.Sprintf("numeric verbosity field %q", logNumeric), func(t *testing.T) {
			for i := 0; i < 2; i++ {
				t.Run(fmt.Sprintf("message verbosity %d", i), func(t *testing.T) {
					var buffer bytes.Buffer
					writer := bufio.NewWriter(&buffer)
					var sampleInfoLogger logr.Logger
					zl := newZapLogger(zapcore.Level(-100), zapcore.AddSync(writer))
					if logNumeric != "" {
						sampleInfoLogger = zapr.NewLoggerWithOptions(zl, zapr.LogInfoLevel(logNumeric))
					} else {
						sampleInfoLogger = zapr.NewLogger(zl)
					}
					sampleInfoLogger.V(i).Info("test", "ns", "default", "podnum", 2, "time", time.Microsecond)
					if err := writer.Flush(); err != nil {
						t.Fatalf("unexpected error from Flush: %v", err)
					}
					logStr := buffer.String()
					var v, lineNo int
					expectedFormat := `{"ts":123.456789,"caller":"zapr/zapr_test.go:%d","msg":"test",` + formatted + `"ns":"default","podnum":2,"time":"1µs"}
`
					expected := ""
					if logNumeric != "" {
						n, err := fmt.Sscanf(logStr, expectedFormat, &lineNo, &v)
						if n != 2 || err != nil {
							t.Errorf("log format error: %d elements, error %s:\n%s", n, err, logStr)
						}
						if v != i {
							t.Errorf("V(%d).Info...) returned v=%d. expected v=%d", i, v, i)
						}
						expected = fmt.Sprintf(expectedFormat, lineNo, v)
					} else {
						n, err := fmt.Sscanf(logStr, expectedFormat, &lineNo)
						if n != 1 || err != nil {
							t.Errorf("log format error: %d elements, error %s:\n%s", n, err, logStr)
						}
						expected = fmt.Sprintf(expectedFormat, lineNo)
					}
					require.JSONEq(t, logStr, expected)
				})
			}
		})
	}
}

// TestError tests Logger.Error.
func TestError(t *testing.T) {
	for _, logError := range []string{"err", "error"} {
		t.Run(fmt.Sprintf("error field name %s", logError), func(t *testing.T) {
			var buffer bytes.Buffer
			writer := bufio.NewWriter(&buffer)
			opts := []zapr.Option{zapr.LogInfoLevel("v")}
			if logError != "error" {
				opts = append(opts, zapr.ErrorKey(logError))
			}
			// Errors always get logged, regardless of log levels.
			var sampleInfoLogger = zapr.NewLoggerWithOptions(newZapLogger(zapcore.Level(-5), zapcore.AddSync(writer)), opts...)
			sampleInfoLogger.V(10).Error(fmt.Errorf("invalid namespace:%s", "default"), "wrong namespace", "ns", "default", "podnum", 2, "time", time.Microsecond)
			if err := writer.Flush(); err != nil {
				t.Fatalf("unexpected error from Flush: %v", err)
			}
			logStr := buffer.String()
			var ts float64
			var lineNo int
			expectedFormat := `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"wrong namespace","ns":"default","podnum":2,"time":"1µs","` + logError + `":"invalid namespace:default"}`
			n, err := fmt.Sscanf(logStr, expectedFormat, &ts, &lineNo)
			if n != 2 || err != nil {
				t.Errorf("log format error: %d elements, error %s:\n%s", n, err, logStr)
			}
			expected := fmt.Sprintf(expectedFormat, ts, lineNo)
			require.JSONEq(t, expected, logStr)
		})
	}
}
