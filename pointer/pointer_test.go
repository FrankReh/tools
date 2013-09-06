// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pointer_test

// This test uses 'expectation' comments embedded within testdata/*.go
// files to specify the expected pointer analysis behaviour.
// See below for grammar.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"code.google.com/p/go.tools/go/types"
	"code.google.com/p/go.tools/go/types/typemap"
	"code.google.com/p/go.tools/importer"
	"code.google.com/p/go.tools/pointer"
	"code.google.com/p/go.tools/ssa"
)

var inputs = []string{
	// Currently debugging:
	// "testdata/tmp.go",

	// Working:
	"testdata/another.go",
	"testdata/arrays.go",
	"testdata/channels.go",
	"testdata/context.go",
	"testdata/conv.go",
	"testdata/flow.go",
	"testdata/fmtexcerpt.go",
	"testdata/func.go",
	"testdata/hello.go",
	"testdata/interfaces.go",
	"testdata/maps.go",
	"testdata/panic.go",
	"testdata/recur.go",
	"testdata/structs.go",
	"testdata/a_test.go",

	// TODO(adonovan): get these tests (of reflection) passing.
	// (The tests are mostly sound since they were used for a
	// previous implementation.)
	// "testdata/funcreflect.go",
	// "testdata/arrayreflect.go",
	// "testdata/chanreflect.go",
	// "testdata/finalizer.go",
	// "testdata/reflect.go",
	// "testdata/mapreflect.go",
	// "testdata/structreflect.go",
}

// Expectation grammar:
//
// @calls f -> g
//
//   A 'calls' expectation asserts that edge (f, g) appears in the
//   callgraph.  f and g are notated as per Function.String(), which
//   may contain spaces (e.g. promoted method in anon struct).
//
// @pointsto a | b | c
//
//   A 'pointsto' expectation asserts that the points-to set of its
//   operand contains exactly the set of labels {a,b,c} notated as per
//   labelString.
//
//   A 'pointsto' expectation must appear on the same line as a
//   print(x) statement; the expectation's operand is x.
//
//   If one of the strings is "...", the expectation asserts that the
//   points-to set at least the other labels.
//
//   We use '|' because label names may contain spaces, e.g.  methods
//   of anonymous structs.
//
//   From a theoretical perspective, concrete types in interfaces are
//   labels too, but they are represented differently and so have a
//   different expectation, @concrete, below.
//
// @concrete t | u | v
//
//   A 'concrete' expectation asserts that the set of possible dynamic
//   types of its interface operand is exactly {t,u,v}, notated per
//   go/types.Type.String(). In other words, it asserts that the type
//   component of the interface may point to that set of concrete type
//   literals.
//
//   A 'concrete' expectation must appear on the same line as a
//   print(x) statement; the expectation's operand is x.
//
//   If one of the strings is "...", the expectation asserts that the
//   interface's type may point to at least the  other concrete types.
//
//   We use '|' because type names may contain spaces.
//
// @warning "regexp"
//
//   A 'warning' expectation asserts that the analysis issues a
//   warning that matches the regular expression within the string
//   literal.
//
// @line id
//
//   A line directive associates the name "id" with the current
//   file:line.  The string form of labels will use this id instead of
//   a file:line, making @pointsto expectations more robust against
//   perturbations in the source file.
//   (NB, anon functions still include line numbers.)
//
type expectation struct {
	kind     string // "pointsto" | "concrete" | "calls" | "warning"
	filename string
	linenum  int // source line number, 1-based
	args     []string
	types    []types.Type // for concrete
}

func (e *expectation) String() string {
	return fmt.Sprintf("@%s[%s]", e.kind, strings.Join(e.args, " | "))
}

func (e *expectation) errorf(format string, args ...interface{}) {
	fmt.Printf("%s:%d: ", e.filename, e.linenum)
	fmt.Printf(format, args...)
	fmt.Println()
}

func (e *expectation) needsProbe() bool {
	return e.kind == "pointsto" || e.kind == "concrete"
}

// A record of a call to the built-in print() function.  Used for testing.
type probe struct {
	instr *ssa.CallCommon
	arg0  pointer.Pointer // first argument to print
}

// Find probe (call to print(x)) of same source
// file/line as expectation.
func findProbe(prog *ssa.Program, probes []probe, e *expectation) *probe {
	for _, p := range probes {
		pos := prog.Fset.Position(p.instr.Pos())
		if pos.Line == e.linenum && pos.Filename == e.filename {
			// TODO(adonovan): send this to test log (display only on failure).
			// fmt.Printf("%s:%d: info: found probe for %s: %s\n",
			// 	e.filename, e.linenum, e, p.arg0) // debugging
			return &p
		}
	}
	return nil // e.g. analysis didn't reach this call
}

func doOneInput(input, filename string) bool {
	impctx := &importer.Config{Build: &build.Default}
	imp := importer.New(impctx)

	// Parsing.
	f, err := parser.ParseFile(imp.Fset, filename, input, parser.DeclarationErrors)
	if err != nil {
		// TODO(adonovan): err is a scanner error list;
		// display all errors not just first?
		fmt.Println(err.Error())
		return false
	}

	// Type checking.
	info, err := imp.CreateSourcePackage("main", []*ast.File{f})
	if err != nil {
		fmt.Println(err.Error())
		return false
	}

	// SSA creation + building.
	prog := ssa.NewProgram(imp.Fset, ssa.SanityCheckFunctions)
	for _, info := range imp.Packages {
		prog.CreatePackage(info)
	}
	prog.BuildAll()

	mainpkg := prog.Package(info.Pkg)
	ptrmain := mainpkg // main package for the pointer analysis
	if mainpkg.Func("main") == nil {
		// No main function; assume it's a test.
		mainpkg.CreateTestMainFunction()
		fmt.Printf("%s: synthesized testmain package for test.\n", imp.Fset.Position(f.Package))
	}

	ok := true

	lineMapping := make(map[string]string) // maps "file:line" to @line tag

	// Parse expectations in this input.
	var exps []*expectation
	re := regexp.MustCompile("// *@([a-z]*) *(.*)$")
	lines := strings.Split(input, "\n")
	for linenum, line := range lines {
		linenum++ // make it 1-based
		if matches := re.FindAllStringSubmatch(line, -1); matches != nil {
			match := matches[0]
			kind, rest := match[1], match[2]
			e := &expectation{kind: kind, filename: filename, linenum: linenum}

			if kind == "line" {
				if rest == "" {
					ok = false
					e.errorf("@%s expectation requires identifier", kind)
				} else {
					lineMapping[fmt.Sprintf("%s:%d", filename, linenum)] = rest
				}
				continue
			}

			if e.needsProbe() && !strings.Contains(line, "print(") {
				ok = false
				e.errorf("@%s expectation must follow call to print(x)", kind)
				continue
			}

			switch kind {
			case "pointsto":
				e.args = split(rest, "|")

			case "concrete":
				for _, typstr := range split(rest, "|") {
					var t types.Type = types.Typ[types.Invalid] // means "..."
					if typstr != "..." {
						texpr, err := parser.ParseExpr(typstr)
						if err != nil {
							ok = false
							// Don't print err since its location is bad.
							e.errorf("'%s' is not a valid type", typstr)
							continue
						}
						t, _, err = types.EvalNode(imp.Fset, texpr, mainpkg.Object, mainpkg.Object.Scope())
						if err != nil {
							ok = false
							// TODO Don't print err since its location is bad.
							e.errorf("'%s' is not a valid type: %s", typstr, err)
							continue
						}
					}
					e.types = append(e.types, t)
				}

			case "calls":
				e.args = split(rest, "->")
				// TODO(adonovan): eagerly reject the
				// expectation if fn doesn't denote
				// existing function, rather than fail
				// the expectation after analysis.
				if len(e.args) != 2 {
					ok = false
					e.errorf("@calls expectation wants 'caller -> callee' arguments")
					continue
				}

			case "warning":
				lit, err := strconv.Unquote(strings.TrimSpace(rest))
				if err != nil {
					ok = false
					e.errorf("couldn't parse @warning operand: %s", err.Error())
					continue
				}
				e.args = append(e.args, lit)

			default:
				ok = false
				e.errorf("unknown expectation kind: %s", e)
				continue
			}
			exps = append(exps, e)
		}
	}

	var probes []probe
	var warnings []string
	var log bytes.Buffer

	callgraph := make(pointer.CallGraph)

	// Run the analysis.
	config := &pointer.Config{
		Mains: []*ssa.Package{ptrmain},
		Log:   &log,
		Print: func(site *ssa.CallCommon, p pointer.Pointer) {
			probes = append(probes, probe{site, p})
		},
		Call: callgraph.AddEdge,
		Warn: func(pos token.Pos, format string, args ...interface{}) {
			msg := fmt.Sprintf(format, args...)
			fmt.Printf("%s: warning: %s\n", prog.Fset.Position(pos), msg)
			warnings = append(warnings, msg)
		},
	}
	pointer.Analyze(config)

	// Print the log is there was an error or a panic.
	complete := false
	defer func() {
		if !complete || !ok {
			log.WriteTo(os.Stderr)
		}
	}()

	// Check the expectations.
	for _, e := range exps {
		var pr *probe
		if e.needsProbe() {
			if pr = findProbe(prog, probes, e); pr == nil {
				ok = false
				e.errorf("unreachable print() statement has expectation %s", e)
				continue
			}
			if pr.arg0 == nil {
				ok = false
				e.errorf("expectation on non-pointerlike operand: %s", pr.instr.Args[0].Type())
				continue
			}
		}

		switch e.kind {
		case "pointsto":
			if !checkPointsToExpectation(e, pr, lineMapping, prog) {
				ok = false
			}

		case "concrete":
			if !checkConcreteExpectation(e, pr) {
				ok = false
			}

		case "calls":
			if !checkCallsExpectation(prog, e, callgraph) {
				ok = false
			}

		case "warning":
			if !checkWarningExpectation(prog, e, warnings) {
				ok = false
			}
		}
	}

	complete = true

	// ok = false // debugging: uncomment to always see log

	return ok
}

func labelString(l *pointer.Label, lineMapping map[string]string, prog *ssa.Program) string {
	// Functions and Globals need no pos suffix.
	switch l.Value.(type) {
	case *ssa.Function, *ssa.Global:
		return l.String()
	}

	str := l.String()
	if pos := l.Value.Pos(); pos != 0 {
		// Append the position, using a @line tag instead of a line number, if defined.
		posn := prog.Fset.Position(l.Value.Pos())
		s := fmt.Sprintf("%s:%d", posn.Filename, posn.Line)
		if tag, ok := lineMapping[s]; ok {
			return fmt.Sprintf("%s@%s:%d", str, tag, posn.Column)
		}
		str = fmt.Sprintf("%s@%s", str, posn)
	}
	return str
}

func checkPointsToExpectation(e *expectation, pr *probe, lineMapping map[string]string, prog *ssa.Program) bool {
	expected := make(map[string]struct{})
	surplus := make(map[string]struct{})
	exact := true
	for _, g := range e.args {
		if g == "..." {
			exact = false
			continue
		}
		expected[g] = struct{}{}
	}
	// Find the set of labels that the probe's
	// argument (x in print(x)) may point to.
	for _, label := range pr.arg0.PointsTo().Labels() {
		name := labelString(label, lineMapping, prog)
		if _, ok := expected[name]; ok {
			delete(expected, name)
		} else if exact {
			surplus[name] = struct{}{}
		}
	}
	// Report set difference:
	ok := true
	if len(expected) > 0 {
		ok = false
		e.errorf("value does not alias these expected labels: %s", join(expected))
	}
	if len(surplus) > 0 {
		ok = false
		e.errorf("value may additionally alias these labels: %s", join(surplus))
	}
	return ok
}

// underlying returns the underlying type of typ.  Copied from go/types.
func underlyingType(typ types.Type) types.Type {
	if typ, ok := typ.(*types.Named); ok {
		return typ.Underlying() // underlying types are never NamedTypes
	}
	if typ == nil {
		panic("underlying(nil)")
	}
	return typ
}

func checkConcreteExpectation(e *expectation, pr *probe) bool {
	var expected typemap.M
	var surplus typemap.M
	exact := true
	for _, g := range e.types {
		if g == types.Typ[types.Invalid] {
			exact = false
			continue
		}
		expected.Set(g, struct{}{})
	}

	switch t := underlyingType(pr.instr.Args[0].Type()).(type) {
	case *types.Interface:
		// ok
	default:
		e.errorf("@concrete expectation requires an interface-typed operand, got %s", t)
		return false
	}

	// Find the set of concrete types that the probe's
	// argument (x in print(x)) may contain.
	for _, conc := range pr.arg0.PointsTo().ConcreteTypes().Keys() {
		if expected.At(conc) != nil {
			expected.Delete(conc)
		} else if exact {
			surplus.Set(conc, struct{}{})
		}
	}
	// Report set difference:
	ok := true
	if expected.Len() > 0 {
		ok = false
		e.errorf("interface cannot contain these concrete types: %s", expected.KeysString())
	}
	if surplus.Len() > 0 {
		ok = false
		e.errorf("interface may additionally contain these concrete types: %s", surplus.KeysString())
	}
	return ok
	return false
}

func checkCallsExpectation(prog *ssa.Program, e *expectation, callgraph pointer.CallGraph) bool {
	// TODO(adonovan): this is inefficient and not robust against
	// typos.  Better to convert strings to *Functions during
	// expectation parsing (somehow).
	for caller, callees := range callgraph {
		if caller.Func().String() == e.args[0] {
			found := make(map[string]struct{})
			for callee := range callees {
				s := callee.Func().String()
				found[s] = struct{}{}
				if s == e.args[1] {
					return true // expectation satisfied
				}
			}
			e.errorf("found no call from %s to %s, but only to %s",
				e.args[0], e.args[1], join(found))
			return false
		}
	}
	e.errorf("didn't find any calls from %s", e.args[0])
	return false
}

func checkWarningExpectation(prog *ssa.Program, e *expectation, warnings []string) bool {
	// TODO(adonovan): check the position part of the warning too?
	re, err := regexp.Compile(e.args[0])
	if err != nil {
		e.errorf("invalid regular expression in @warning expectation: %s", err.Error())
		return false
	}

	if len(warnings) == 0 {
		e.errorf("@warning %s expectation, but no warnings", strconv.Quote(e.args[0]))
		return false
	}

	for _, warning := range warnings {
		if re.MatchString(warning) {
			return true
		}
	}

	e.errorf("@warning %s expectation not satised; found these warnings though:", strconv.Quote(e.args[0]))
	for _, warning := range warnings {
		fmt.Println("\t", warning)
	}
	return false
}

func TestInput(t *testing.T) {
	ok := true

	wd, err := os.Getwd()
	if err != nil {
		t.Errorf("os.Getwd: %s", err.Error())
		return
	}

	// 'go test' does a chdir so that relative paths in
	// diagnostics no longer make sense relative to the invoking
	// shell's cwd.  We print a special marker so that Emacs can
	// make sense of them.
	fmt.Fprintf(os.Stderr, "Entering directory `%s'\n", wd)

	for _, filename := range inputs {
		content, err := ioutil.ReadFile(filename)
		if err != nil {
			t.Errorf("couldn't read file '%s': %s", filename, err.Error())
			continue
		}

		if !doOneInput(string(content), filename) {
			ok = false
		}
	}
	if !ok {
		t.Fail()
	}
}

// join joins the elements of set with " | "s.
func join(set map[string]struct{}) string {
	var buf bytes.Buffer
	sep := ""
	for name := range set {
		buf.WriteString(sep)
		sep = " | "
		buf.WriteString(name)
	}
	return buf.String()
}

// split returns the list of sep-delimited non-empty strings in s.
func split(s, sep string) (r []string) {
	for _, elem := range strings.Split(s, sep) {
		elem = strings.TrimSpace(elem)
		if elem != "" {
			r = append(r, elem)
		}
	}
	return
}