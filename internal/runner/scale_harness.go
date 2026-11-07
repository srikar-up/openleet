package runner

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"openleet/internal/model"
)

type scaleHarness struct{}

func newScaleHarness() *scaleHarness {
	return &scaleHarness{}
}

// Generate creates the runnable harness files for scaling-curve benchmarking.
func (h *scaleHarness) Generate(language string, problem *model.Problem, code string, cases map[string][]model.TestCase, repeat int) (map[string]string, string, error) {
	userCode := code
	if userCode == "" && problem != nil && problem.Templates != nil {
		userCode = problem.Templates[language]
	}

	switch strings.ToLower(strings.TrimSpace(language)) {
	case "python":
		return h.generatePython(problem, userCode, cases, repeat)
	case "go":
		return h.generateGo(problem, userCode, cases, repeat)
	default:
		return nil, "", fmt.Errorf("unsupported language for scale harness: %s", language)
	}
}

func (h *scaleHarness) generatePython(problem *model.Problem, userCode string, cases map[string][]model.TestCase, repeat int) (map[string]string, string, error) {
	casesJSON, err := json.Marshal(cases)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal scale test cases: %w", err)
	}

	slug := ""
	if problem != nil {
		slug = problem.Slug
	}

	var b strings.Builder
	b.WriteString(`#!/usr/bin/env python3
import copy
import inspect
import json
import sys
import time
import traceback

_cases_raw = `)
	b.WriteString(strconv.Quote(string(casesJSON)))
	b.WriteString(`
_repeat = `)
	b.WriteString(strconv.Itoa(repeat))
	b.WriteString(`
_scales_data = json.loads(_cases_raw)

# ==================== USER SOLUTION BEGIN ====================
`)
	b.WriteString(userCode)
	b.WriteString(`
# ==================== USER SOLUTION END ======================

def _norm(name):
    return name.replace("_", "").replace("-", "").lower()

_slug_key = _norm(`)
	b.WriteString(strconv.Quote(slug))
	b.WriteString(`)

def _resolve_callable():
    if 'Solution' in globals() and inspect.isclass(Solution):
        instance = Solution()
        methods = [m for m in dir(instance) if not m.startswith('_') and callable(getattr(instance, m))]
        if methods:
            for m in methods:
                if _norm(m) == _slug_key:
                    return getattr(instance, m)
            return getattr(instance, methods[0])

    for candidate in [`)
	b.WriteString(strconv.Quote(slug))
	b.WriteString(`, "solution"]:
        if candidate in globals() and callable(globals()[candidate]):
            return globals()[candidate]

    for k, v in globals().items():
        if not k.startswith('_') and callable(v) and inspect.isfunction(v):
            if _norm(k) == _slug_key:
                return v

    for k, v in globals().items():
        if not k.startswith('_') and callable(v) and inspect.isfunction(v):
            return v

    raise RuntimeError("No solution callable found. Please define 'class Solution' or function matching problem.")

_fn = _resolve_callable()
_output = {"scales": {}}

for _scale_name, _cases in _scales_data.items():
    _case_rows = []
    for _tc in _cases:
        _tc_id = _tc.get("id", "")
        _expected = _tc.get("expected")
        _raw_inp = _tc.get("input")
        _best_ns = -1
        _last_actual = None
        _err = ""

        for _r in range(_repeat):
            _inp = copy.deepcopy(_raw_inp)
            _t0 = time.perf_counter_ns()
            try:
                if isinstance(_inp, dict):
                    _actual = _fn(**_inp)
                elif isinstance(_inp, list):
                    _actual = _fn(*_inp)
                else:
                    _actual = _fn(_inp)
            except Exception:
                _err = traceback.format_exc()
                break
            _elapsed = time.perf_counter_ns() - _t0
            if _elapsed <= 0:
                _elapsed = 1
            _last_actual = _actual
            if _best_ns < 0 or _elapsed < _best_ns:
                _best_ns = _elapsed

        if _err != "":
            _case_rows.append({
                "test_case_id": _tc_id,
                "passed": False,
                "actual": None,
                "expected": _expected,
                "error": _err,
                "duration_ns": 0,
            })
        else:
            _passed = (_last_actual == _expected)
            _case_rows.append({
                "test_case_id": _tc_id,
                "passed": _passed,
                "actual": _last_actual,
                "expected": _expected,
                "error": "",
                "duration_ns": _best_ns,
            })
    _output["scales"][_scale_name] = {"cases": _case_rows}

sys.stdout.write("__OPENLEET_SCALE__" + json.dumps(_output) + "\n")
sys.stdout.flush()
`)

	files := map[string]string{
		"solution.py": b.String(),
	}
	return files, "solution.py", nil
}

func (h *scaleHarness) generateGo(problem *model.Problem, userCode string, cases map[string][]model.TestCase, repeat int) (map[string]string, string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "user.go", userCode, 0)
	if err != nil && !strings.Contains(userCode, "package ") {
		userCode = "package main\n\n" + userCode
		node, err = parser.ParseFile(fset, "user.go", userCode, 0)
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse user Go code: %w", err)
	}
	if node.Name == nil || node.Name.Name != "main" {
		return nil, "", fmt.Errorf("Go solution must use 'package main'")
	}

	type candidateFunc struct {
		decl *ast.FuncDecl
		name string
	}
	var candidates []candidateFunc
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		name := fn.Name.Name
		if name == "main" || name == "init" {
			continue
		}
		candidates = append(candidates, candidateFunc{decl: fn, name: name})
	}

	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("no user function found in Go code")
	}

	slug := ""
	if problem != nil {
		slug = problem.Slug
	}
	camelSlug := camelCase(slug)

	var chosen *ast.FuncDecl
	for _, c := range candidates {
		if c.name == camelSlug {
			chosen = c.decl
			break
		}
	}
	if chosen == nil && slug != "" {
		for _, c := range candidates {
			if c.name == slug {
				chosen = c.decl
				break
			}
		}
	}
	if chosen == nil {
		for _, c := range candidates {
			if c.name == "Solve" {
				chosen = c.decl
				break
			}
		}
	}
	if chosen == nil {
		chosen = candidates[0].decl
	}

	// Check if variadic
	if chosen.Type.Params != nil {
		for _, field := range chosen.Type.Params.List {
			if _, ok := field.Type.(*ast.Ellipsis); ok {
				return nil, "", fmt.Errorf("variadic functions are not supported: %s", chosen.Name.Name)
			}
		}
	}

	// Extract parameter names
	var paramNames []string
	if chosen.Type.Params != nil {
		for _, field := range chosen.Type.Params.List {
			if len(field.Names) == 0 {
				paramNames = append(paramNames, "")
			} else {
				for _, ident := range field.Names {
					paramNames = append(paramNames, ident.Name)
				}
			}
		}
	}

	casesJSON, err := json.Marshal(cases)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal scale test cases: %w", err)
	}

	quotedParams := make([]string, len(paramNames))
	for i, p := range paramNames {
		quotedParams[i] = strconv.Quote(p)
	}
	paramsSliceCode := strings.Join(quotedParams, ", ")

	var b strings.Builder
	b.WriteString(`package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"
)

type scaleCase struct {
	ID       string      ` + "`json:\"id\"`" + `
	Input    interface{} ` + "`json:\"input\"`" + `
	Expected interface{} ` + "`json:\"expected\"`" + `
}

type caseRow struct {
	TestCaseID string      ` + "`json:\"test_case_id\"`" + `
	Passed     bool        ` + "`json:\"passed\"`" + `
	Actual     interface{} ` + "`json:\"actual\"`" + `
	Expected   interface{} ` + "`json:\"expected\"`" + `
	Error      string      ` + "`json:\"error,omitempty\"`" + `
	DurationNs int64       ` + "`json:\"duration_ns\"`" + `
}

type scaleRow struct {
	Cases []caseRow ` + "`json:\"cases\"`" + `
}

type scaleOutput struct {
	Scales map[string]scaleRow ` + "`json:\"scales\"`" + `
}

var scaleCasesJSON = `)
	b.WriteString(strconv.Quote(string(casesJSON)))
	b.WriteString(`
var scaleRepeat = `)
	b.WriteString(strconv.Itoa(repeat))
	b.WriteString(`
var userFunc = `)
	b.WriteString(chosen.Name.Name)
	b.WriteString(`
var userFuncParams = []string{` + paramsSliceCode + `}

func checkEqual(actual, expected interface{}) bool {
	actBytes, err1 := json.Marshal(actual)
	expBytes, err2 := json.Marshal(expected)
	if err1 == nil && err2 == nil {
		if bytes.Equal(actBytes, expBytes) {
			return true
		}
		var aNorm, eNorm interface{}
		if json.Unmarshal(actBytes, &aNorm) == nil && json.Unmarshal(expBytes, &eNorm) == nil {
			return reflect.DeepEqual(aNorm, eNorm)
		}
	}
	return reflect.DeepEqual(actual, expected)
}

func invoke(fn reflect.Value, args []reflect.Value) (actual interface{}, errStr string) {
	defer func() {
		if r := recover(); r != nil {
			errStr = fmt.Sprintf("panic: %v", r)
		}
	}()
	res := fn.Call(args)
	if len(res) > 0 {
		actual = res[0].Interface()
	}
	return actual, ""
}

func buildArgs(fn reflect.Value, rawInput interface{}) ([]reflect.Value, error) {
	fnType := fn.Type()
	numIn := fnType.NumIn()
	if numIn == 0 {
		return nil, nil
	}
	argVals := make([]reflect.Value, numIn)

	coerce := func(val interface{}, target reflect.Type) (reflect.Value, error) {
		data, err := json.Marshal(val)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("marshal arg: %w", err)
		}
		ptr := reflect.New(target)
		if err := json.Unmarshal(data, ptr.Interface()); err != nil {
			return reflect.Value{}, fmt.Errorf("unmarshal arg into %v: %w", target, err)
		}
		return ptr.Elem(), nil
	}

	if rawMap, ok := rawInput.(map[string]interface{}); ok {
		allMatch := numIn > 0
		for _, name := range userFuncParams {
			if name == "" {
				allMatch = false
				break
			}
			if _, exists := rawMap[name]; !exists {
				allMatch = false
				break
			}
		}
		if allMatch {
			for i := 0; i < numIn; i++ {
				cv, err := coerce(rawMap[userFuncParams[i]], fnType.In(i))
				if err != nil {
					return nil, fmt.Errorf("param %q: %w", userFuncParams[i], err)
				}
				argVals[i] = cv
			}
			return argVals, nil
		}
		if numIn == 1 {
			cv, err := coerce(rawInput, fnType.In(0))
			if err != nil {
				return nil, fmt.Errorf("single param map: %w", err)
			}
			argVals[0] = cv
			return argVals, nil
		}
		return nil, fmt.Errorf("mismatched argument map: could not map keys to function parameters")
	}

	if rawSlice, ok := rawInput.([]interface{}); ok {
		if len(rawSlice) == numIn {
			for i := 0; i < numIn; i++ {
				cv, err := coerce(rawSlice[i], fnType.In(i))
				if err != nil {
					return nil, fmt.Errorf("positional arg %d: %w", i, err)
				}
				argVals[i] = cv
			}
			return argVals, nil
		}
		if numIn == 1 {
			cv, err := coerce(rawInput, fnType.In(0))
			if err != nil {
				return nil, fmt.Errorf("single param slice: %w", err)
			}
			argVals[0] = cv
			return argVals, nil
		}
		return nil, fmt.Errorf("mismatched argument count: expected %d, got %d", numIn, len(rawSlice))
	}

	if numIn == 1 {
		cv, err := coerce(rawInput, fnType.In(0))
		if err != nil {
			return nil, fmt.Errorf("single scalar param: %w", err)
		}
		argVals[0] = cv
		return argVals, nil
	}

	return nil, fmt.Errorf("mismatched arguments: function expects %d parameters", numIn)
}

func runCase(fn reflect.Value, tc scaleCase) caseRow {
	var bestNs int64 = -1
	var lastActual interface{}
	var caseErr string

	for r := 0; r < scaleRepeat; r++ {
		var clonedInput interface{}
		inpBytes, err := json.Marshal(tc.Input)
		if err != nil {
			caseErr = fmt.Sprintf("clone marshal: %v", err)
			break
		}
		if err := json.Unmarshal(inpBytes, &clonedInput); err != nil {
			caseErr = fmt.Sprintf("clone unmarshal: %v", err)
			break
		}

		args, err := buildArgs(fn, clonedInput)
		if err != nil {
			caseErr = err.Error()
			break
		}

		t0 := time.Now()
		act, errStr := invoke(fn, args)
		elapsed := time.Since(t0).Nanoseconds()
		if elapsed <= 0 {
			elapsed = 1
		}

		if errStr != "" {
			caseErr = errStr
			break
		}

		lastActual = act
		if bestNs < 0 || elapsed < bestNs {
			bestNs = elapsed
		}
	}

	if caseErr != "" {
		return caseRow{
			TestCaseID: tc.ID,
			Passed:     false,
			Actual:     nil,
			Expected:   tc.Expected,
			Error:      caseErr,
			DurationNs: 0,
		}
	}

	passed := checkEqual(lastActual, tc.Expected)
	return caseRow{
		TestCaseID: tc.ID,
		Passed:     passed,
		Actual:     lastActual,
		Expected:   tc.Expected,
		Error:      "",
		DurationNs: bestNs,
	}
}

func main() {
	var casesMap map[string][]scaleCase
	if err := json.Unmarshal([]byte(scaleCasesJSON), &casesMap); err != nil {
		fmt.Fprintf(os.Stderr, "failed to unmarshal scale cases: %v\n", err)
		os.Exit(1)
	}

	fn := reflect.ValueOf(userFunc)
	out := scaleOutput{
		Scales: make(map[string]scaleRow),
	}

	for _, scale := range []string{"small", "medium", "large"} {
		if cases, ok := casesMap[scale]; ok {
			rows := make([]caseRow, len(cases))
			for i, tc := range cases {
				rows[i] = runCase(fn, tc)
			}
			out.Scales[scale] = scaleRow{Cases: rows}
		}
	}
	for scale, cases := range casesMap {
		if _, ok := out.Scales[scale]; !ok {
			rows := make([]caseRow, len(cases))
			for i, tc := range cases {
				rows[i] = runCase(fn, tc)
			}
			out.Scales[scale] = scaleRow{Cases: rows}
		}
	}

	b, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal output: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("__OPENLEET_SCALE__%s\n", string(b))
}
`)

	files := map[string]string{
		"main.go": b.String(),
		"user.go": userCode,
	}
	return files, "main.go", nil
}

func camelCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-'
	})
	if len(parts) == 0 {
		return s
	}
	var b strings.Builder
	first := true
	for _, part := range parts {
		if part == "" {
			continue
		}
		if first {
			b.WriteString(strings.ToLower(part))
			first = false
		} else {
			runes := []rune(part)
			b.WriteString(strings.ToUpper(string(runes[0])))
			if len(runes) > 1 {
				b.WriteString(strings.ToLower(string(runes[1:])))
			}
		}
	}
	return b.String()
}
