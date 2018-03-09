// Copyright 2018 Frank Rehwinkel
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import (
	"bytes"
	"fmt"
	"sort"
)

// buildBitflag generates the variables and String method for bitflag values.
func (g *Generator) buildBitflag(values []Value, typeName string) {
	zero, runs := splitIntoBitflagRuns(values)

	zeroName := typeName + "(0)"
	if zero != nil {
		zeroName = zero.name
	}

	switch {
	case len(runs) == 1:
		g.buildOneRunBitflag(runs, typeName, zeroName)
	case maxGap(runs) <= 4:
		g.buildOneRunBitflagShortGaps(runs, typeName, zeroName)
	default:
		g.buildOneRunBitflagLongGaps(runs, typeName, zeroName)
	}
}

// splitIntoBitflagRuns sorts values from lowest to highest, removing
// duplicates (and multi-bit flag for now).  The zero value and the runs are
// returned.  The input slice is known to be non-empty and is modified in
// place.
func splitIntoBitflagRuns(values []Value) (*Value, [][]Value) {

	// If any are signed, this is probably messed up. Just drop the sign.
	for i := range values {
		values[i].signed = false
	}
	// We use stable sort so the lexically first name is chosen for equal elements.
	sort.Stable(byValue(values))

	var zero *Value
	if values[0].value == 0 {
		zero = &values[0]
		for values[0].value == 0 {
			values = values[1:]
		}
	}

	// Remove duplicates. Stable sort has put the one we want to print first,
	// so use that one. Any zero values have been removed from the front.
	// Also remove any with multiple bits set.
	j := 1
	for i := 1; i < len(values); i++ {
		if values[i].value != values[i-1].value && singleBitSet(values[i].value) {
			values[j] = values[i]
			j++
		}
	}
	values = values[:j]
	runs := make([][]Value, 0, 10)
	for len(values) > 0 {
		// One contiguous sequence per outer loop.
		i := 1
		for i < len(values) && values[i].value == values[i-1].value<<1 {
			i++
		}
		runs = append(runs, values[:i])
		values = values[i:]
	}
	return zero, runs
}

// maxGap returns the max number of bitflag constants missing between runs of consecutive bitflags.
func maxGap(runs [][]Value) int {
	m := 0

	for r := range runs[:len(runs)-1] {
		gapHead := runLastValue(runs[r]).value
		gapTail := runs[r+1][0].value
		g := 0
		gapHead <<= 1
		for gapHead < gapTail {
			g++
			gapHead <<= 1
		}
		if g > m {
			m = g
		}
	}

	return m
}

// buildOneRunBitflag generates the variables and String method for a single run of contiguous values.
func (g *Generator) buildOneRunBitflag(runs [][]Value, typeName, zeroName string) {
	values := runs[0]
	g.Printf("\n")
	//g.declareIndexAndNameVar(values, typeName)

	index, name := g.createIndexAndNameDecl(values, typeName, "")
	g.declareIndex(typeName, name, index, "")

	// The generated code is simple enough to write as a Printf format.
	spanGapCheck := ""
	initialValue := values[0].String()
	if g.cache {
		g.Printf(stringBitflagCacheCode, typeName, usize(len(values)), zeroName, spanGapCheck, initialValue, 256)
	} else {
		g.Printf(stringBitflagCode, typeName, usize(len(values)), zeroName, spanGapCheck, initialValue, 0)
	}
}

// buildOneRunBitflagShortGaps generates the variables and String method for runs with short gaps in between.
func (g *Generator) buildOneRunBitflagShortGaps(runs [][]Value, typeName, zeroName string) {
	values := runs[0]
	g.Printf("\n")

	index, name := g.createIndexAndNameDeclShortGapBitflags(runs, typeName, "")
	g.declareIndex(typeName, name, index, "")

	// The generated code is simple enough to write as a Printf format.
	spanGapCheck := "p0 == p1 || "
	initialValue := values[0].String()
	if g.cache {
		g.Printf(stringBitflagCacheCode, typeName, usize(len(values)), zeroName, spanGapCheck, initialValue, 256)
	} else {
		g.Printf(stringBitflagCode, typeName, usize(len(values)), zeroName, spanGapCheck, initialValue, 0)
	}
}

// buildOneRunBitflagLongGaps generates the variables and String method for runs with long gaps in between.
func (g *Generator) buildOneRunBitflagLongGaps(runs [][]Value, typeName, zeroName string) {
	values := runs[0]
	g.Printf("\n")
	index, skip, name := g.createIndexAndNameDeclLongGapsBitflags(runs, typeName, "")
	g.declareIndex(typeName, name, index, skip)

	// The generated code is simple enough to write as a Printf format.
	spanGapCheck := ""
	initialValue := values[0].String()
	if g.cache {
		g.Printf(stringBitflagCacheCodeLongGaps, typeName, usize(len(values)), zeroName, spanGapCheck, initialValue, 256)
	} else {
		g.Printf(stringBitflagCodeLongGaps, typeName, usize(len(values)), zeroName, spanGapCheck, initialValue, 0)
	}
}

// declareIndex
func (g *Generator) declareIndex(typeName, name, index, skip string) {
	g.Printf("const %s\n", name)
	if !g.cache && skip == "" {
		g.Printf("var %s\n", index)
		return
	}
	g.Printf("var (\n")
	g.Printf("\t%s\n", index)
	if skip != "" {
		g.Printf("\t%s\n", skip)
	}
	if g.cache {
		g.Printf("\t_%[1]s_cache = make(map[%[1]s]string)\n", typeName)
		g.Printf("\t_%[1]s_cachemu sync.Mutex\n", typeName)
	}
	g.Printf(")\n\n")
}

// createIndexAndNameDeclShortGapBitflags returns the pair of declarations for the runs. The caller will add "const" and "var".
func (g *Generator) createIndexAndNameDeclShortGapBitflags(runs [][]Value, typeName string, suffix string) (string, string) {
	b := new(bytes.Buffer)
	var indexes []int
	for r, run := range runs {
		for i := range run {
			b.WriteString(run[i].name)
			indexes = append(indexes, b.Len())
		}

		if r == len(runs)-1 {
			continue
		}
		// Handle short gaps by duplicating the last index as many times as necessary.
		gapHead := runLastValue(run).value
		gapTail := runs[r+1][0].value
		gapHead <<= 1
		for gapHead < gapTail {
			indexes = append(indexes, indexes[len(indexes)-1])
			gapHead <<= 1
		}
	}
	nameConst := fmt.Sprintf("_%s_name%s = %q", typeName, suffix, b.String())
	nameLen := b.Len()
	b.Reset()
	fmt.Fprintf(b, "_%s_index%s = [...]uint%d{0, ", typeName, suffix, usize(nameLen))
	for i, v := range indexes {
		if i > 0 {
			fmt.Fprintf(b, ", ")
		}
		fmt.Fprintf(b, "%d", v)
	}
	fmt.Fprintf(b, "}")
	return b.String(), nameConst
}

// createIndexAndNameDeclLongGapsBitflags returns the pair of declarations for the runs. The caller will add "const" and "var".
func (g *Generator) createIndexAndNameDeclLongGapsBitflags(runs [][]Value, typeName string, suffix string) (string, string, string) {
	b := new(bytes.Buffer)
	var indexes []int
	var skips []int
	for r, run := range runs {
		for i := range run {
			b.WriteString(run[i].name)
			indexes = append(indexes, b.Len())
		}

		if r == len(runs)-1 {
			continue
		}
		// Handle each by placing a single duplicate entry in the indexes list
		// and placing the skip amount in the skips list.
		gapHead := runLastValue(run).value
		gapTail := runs[r+1][0].value
		skipAmount := 0
		gapHead <<= 1
		for gapHead < gapTail {
			skipAmount++
			gapHead <<= 1
		}
		skips = append(skips, skipAmount)
		indexes = append(indexes, indexes[len(indexes)-1])
	}
	nameConst := fmt.Sprintf("_%s_name%s = %q", typeName, suffix, b.String())

	b.Reset()
	fmt.Fprintf(b, "_%s_skips = [...]uint8{", typeName)
	for i, v := range skips {
		if i > 0 {
			fmt.Fprintf(b, ", ")
		}
		fmt.Fprintf(b, "%d", v)
	}
	fmt.Fprintf(b, "}")
	skipsStr := b.String()
	b.Reset()

	fmt.Fprintf(b, "_%s_index%s = [...]uint%d{0, ", typeName, suffix, usize(len(nameConst)))
	for i, v := range indexes {
		if i > 0 {
			fmt.Fprintf(b, ", ")
		}
		fmt.Fprintf(b, "%d", v)
	}
	fmt.Fprintf(b, "}")
	return b.String(), skipsStr, nameConst
}

func runLastValue(run []Value) *Value {
	return &run[len(run)-1]
}

// singleBitSet returns true when one and only one bit is set in the value v.
func singleBitSet(v uint64) bool {
	return v != 0 && (v&(v-1)) == 0
}

// Arguments to format are:
//	[1]: type name
//	[2]: size of index element (8 for uint8 etc.)
//	[3]: zeroName
//	[4]: span gap check : "" or "p0 == p1 || "
//	[5]: initial value : example "(1)"
//	[6]: max cache size
const stringBitflagCode = `func (m %[1]s) String() string {
	if m == 0 {
		return "%[3]s"
	}

	var b []byte
	l := len(_%[1]s_index)
	v := %[1]s(%[5]s)
	p1 := _%[1]s_index[0]
	p0 := p1
	for i := 1; i < l; i, v = i+1, v<<1 {
		p0, p1 = p1, _%[1]s_index[i]
		if %[4]sv&m == 0 {
			continue
		}
		m ^= v
		if len(b) == 0 {
			if m == 0 {
				return _%[1]s_name[p0:p1]
			}
			b = append(b, '(')
		} else {
			b = append(b, '|')
		}
		b = append(b, _%[1]s_name[p0:p1]...)
		if m == 0 {
			b = append(b, ')')
			return string(b)
		}
	}
	s := "%[1]s(0x" + strconv.FormatUint(uint64(m), 16) + ")"
	if len(b) == 0 {
		return s
	}
	b = append(b, '|')
	b = append(b, s...)
	b = append(b, ')')
	return string(b)
}
`

// Arguments to format are:
//	[1]: type name
//	[2]: size of index element (8 for uint8 etc.)
//	[3]: zeroName
//	[4]: span gap check : "" or "p0 == p1 || "
//	[5]: initial value : example "(1)"
//	[6]: cache size limit
const stringBitflagCacheCode = `func (m %[1]s) String() string {
	_%[1]s_cachemu.Lock()
	s, ok := _%[1]s_cache[m]
	_%[1]s_cachemu.Unlock()
	if ok {
		return s
	}
	s = m._string()
	_%[1]s_cachemu.Lock()
	if len(_%[1]s_cache) >= %[6]d {
		_%[1]s_cache = make(map[%[1]s]string, %[6]d)
	}
	_%[1]s_cache[m] = s
	_%[1]s_cachemu.Unlock()
	return s
}

func (m %[1]s) _string() string {
	if m == 0 {
		return "%[3]s"
	}

	var b []byte
	l := len(_%[1]s_index)
	v := %[1]s(%[5]s)
	p1 := _%[1]s_index[0]
	p0 := p1
	for i := 1; i < l; i, v = i+1, v<<1 {
		p0, p1 = p1, _%[1]s_index[i]
		if %[4]sv&m == 0 {
			continue
		}
		m ^= v
		if len(b) == 0 {
			if m == 0 {
				return _%[1]s_name[p0:p1]
			}
			b = append(b, '(')
		} else {
			b = append(b, '|')
		}
		b = append(b, _%[1]s_name[p0:p1]...)
		if m == 0 {
			b = append(b, ')')
			return string(b)
		}
	}
	s := "%[1]s(0x" + strconv.FormatUint(uint64(m), 16) + ")"
	if len(b) == 0 {
		return s
	}
	b = append(b, '|')
	b = append(b, s...)
	b = append(b, ')')
	return string(b)
}
`

// Arguments to format are:
//	[1]: type name
//	[2]: size of index element (8 for uint8 etc.)
//	[3]: zeroName
//	[4]: span gap check - a noop for this format
//	[5]: initial value : example "(1)"
//	[6]: cache size limit
const stringBitflagCodeLongGaps = `func (m %[1]s) String() string {
	if m == 0 {
		return "%[3]s"
	}

	var b []byte
	l := len(_%[1]s_index)
	si := 0
	v := %[1]s(%[5]s)
	p1 := _%[1]s_index[0]
	p0 := p1
	for i := 1; i < l; i, v = i+1, v<<1 {
		p0, p1 = p1, _%[1]s_index[i]
		if p0 == p1 {
			v <<= _%[1]s_skips[si] - 1
			si++
			continue
		}
		if v&m == 0 {
			continue
		}
		m ^= v
		if len(b) == 0 {
			if m == 0 {
				return _%[1]s_name[p0:p1]
			}
			b = append(b, '(')
		} else {
			b = append(b, '|')
		}
		b = append(b, _%[1]s_name[p0:p1]...)
		if m == 0 {
			b = append(b, ')')
			return string(b)
		}
	}
	s := "%[1]s(0x" + strconv.FormatUint(uint64(m), 16) + ")"
	if len(b) == 0 {
		return s
	}
	b = append(b, '|')
	b = append(b, s...)
	b = append(b, ')')
	return string(b)
}
`

// Arguments to format are:
//	[1]: type name
//	[2]: size of index element (8 for uint8 etc.)
//	[3]: zeroName
//	[4]: span gap check - a noop for this format
//	[5]: initial value : example "(1)"
//	[6]: cache size limit
const stringBitflagCacheCodeLongGaps = `func (m %[1]s) String() string {
	_%[1]s_cachemu.Lock()
	s, ok := _%[1]s_cache[m]
	_%[1]s_cachemu.Unlock()
	if ok {
		return s
	}
	s = m._string()
	_%[1]s_cachemu.Lock()
	if len(_%[1]s_cache) >= %[6]d {
		_%[1]s_cache = make(map[%[1]s]string, %[6]d)
	}
	_%[1]s_cache[m] = s
	_%[1]s_cachemu.Unlock()
	return s
}

func (m %[1]s) _string() string {
	if m == 0 {
		return "%[3]s"
	}

	var b []byte
	l := len(_%[1]s_index)
	si := 0
	v := %[1]s(%[5]s)
	p1 := _%[1]s_index[0]
	p0 := p1
	for i := 1; i < l; i, v = i+1, v<<1 {
		p0, p1 = p1, _%[1]s_index[i]
		if p0 == p1 {
			v <<= _%[1]s_skips[si] - 1
			si++
			continue
		}
		if v&m == 0 {
			continue
		}
		m ^= v
		if len(b) == 0 {
			if m == 0 {
				return _%[1]s_name[p0:p1]
			}
			b = append(b, '(')
		} else {
			b = append(b, '|')
		}
		b = append(b, _%[1]s_name[p0:p1]...)
		if m == 0 {
			b = append(b, ')')
			return string(b)
		}
	}
	s := "%[1]s(0x" + strconv.FormatUint(uint64(m), 16) + ")"
	if len(b) == 0 {
		return s
	}
	b = append(b, '|')
	b = append(b, s...)
	b = append(b, ')')
	return string(b)
}
`
