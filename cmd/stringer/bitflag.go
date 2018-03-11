// Copyright 2018 Frank Rehwinkel
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// These routines implement the bulk of the bitflag code generation for stringer.

package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
)

// buildBitflag generates the variables and String method for bitflag values.
func (g *Generator) buildBitflag(values []Value, typeName string) {
	zero, runs := splitIntoBitflagRuns(values)

	zeroName := typeName + "(0)"
	if zero != nil {
		zeroName = zero.name
	}
	initialValue := runs[0][0].String()

	name, offsets, skips := g.nameAndRest(runs)

	name = fmt.Sprintf("_%s_name = %q", typeName, name)

	g.Printf("\n")
	g.declareNameAndRest(typeName, name, offsets, skips)

	if g.cache {
		if len(skips) == 0 {
			g.Printf(stringBitflagCacheCode, typeName, 0, zeroName, 0, initialValue, 256)
		} else {
			g.Printf(stringBitflagCacheCodeWithSkips, typeName, 0, zeroName, 0, initialValue, 256)
		}
	} else {
		if len(skips) == 0 {
			g.Printf(stringBitflagCode, typeName, 0, zeroName, 0, initialValue, 0)
		} else {
			g.Printf(stringBitflagCodeWithSkips, typeName, 0, zeroName, 0, initialValue, 0)
		}
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

// singleBitSet returns true when one and only one bit is set in the value v.
func singleBitSet(v uint64) bool {
	return v != 0 && (v&(v-1)) == 0
}

// nameAndRest returns the name string for the runs, and the list of offsets and skips.
func (g *Generator) nameAndRest(runs [][]Value) (name string, offsets []int, skips []int) {
	var names []string
	for r, run := range runs {
		for i := range run {
			n := run[i].name
			o := len(n)

			names = append(names, n)
			if o > 255 {
				fmt.Fprintf(os.Stderr, "stringer: name too long (%d): %s\n", o, n)

				os.Exit(1)
			}
			offsets = append(offsets, o)
		}

		if r < len(runs)-1 {
			// Handle gap to next run by appending the skip amount in the skips
			// list and appending a zero to the offsets list.
			skips = append(skips, shiftCount(runLastValue(run).value, runs[r+1][0].value)-1)
			offsets = append(offsets, 0) // 0 will signal to skip.
		}
	}
	name = strings.Join(names, "")
	return
}

// shiftCount returns number of times prev needs to be shifted to meet or exceed next.
func shiftCount(prev, next uint64) int {
	count := 0
	for prev < next {
		count++
		prev <<= 1
	}
	return count
}

func runLastValue(run []Value) *Value {
	return &run[len(run)-1]
}

// declareNameAndRest
func (g *Generator) declareNameAndRest(typeName, name string, offsets, skips []int) {
	offset := fmt.Sprintf("_%s_offset = [...]uint8{%s}", typeName, intString(offsets))

	g.Printf("const %s\n", name)
	if !g.cache && len(skips) == 0 {
		g.Printf("var %s\n", offset)
		return
	}
	g.Printf("var (\n")
	g.Printf("\t%s\n", offset)
	if len(skips) != 0 {
		g.Printf("\t_%s_skips = [...]uint8{%s}\n", typeName, intString(skips))
	}
	if g.cache {
		g.Printf("\t_%[1]s_cache = make(map[%[1]s]string)\n", typeName)
		g.Printf("\t_%[1]s_cachemu sync.Mutex\n", typeName)
	}
	g.Printf(")\n\n")
}

// intString returns the string of int values
func intString(values []int) string {
	r := new(bytes.Buffer)
	sep := ""
	for _, v := range values {
		fmt.Fprintf(r, "%s%d", sep, v)
		sep = ", "
	}
	return r.String()
}

// Arguments to format are:
//	[1]: type name
//	[2]: 0 a noop
//	[3]: zeroName
//	[4]: 0 a noop
//	[5]: initial value : example "(1)"
//	[6]: max cache size
const stringBitflagCode = `func (m %[1]s) String() string {
	if m == 0 {
		return "%[3]s"
	}

	var b []byte
	l := len(_%[1]s_offset)
	v := %[1]s(%[5]s)
	p0 := 0
	p1 := 0
	for i := 0; i < l; i, v = i+1, v<<1 {
		p0 = p1
		p1 += int(_%[1]s_offset[i])
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
//	[2]: 0 a noop
//	[3]: zeroName
//	[4]: 0 a noop
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
	l := len(_%[1]s_offset)
	v := %[1]s(%[5]s)
	p0 := 0
	p1 := 0
	for i := 0; i < l; i, v = i+1, v<<1 {
		p0 = p1
		p1 += int(_%[1]s_offset[i])
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
//	[2]: 0 a noop
//	[3]: zeroName
//	[4]: 0 a noop
//	[5]: initial value : example "(1)"
//	[6]: cache size limit
const stringBitflagCodeWithSkips = `func (m %[1]s) String() string {
	if m == 0 {
		return "%[3]s"
	}

	var b []byte
	l := len(_%[1]s_offset)
	v := %[1]s(%[5]s)
	si := 0
	p0 := 0
	p1 := 0
	for i := 0; i < l; i, v = i+1, v<<1 {
		o := _%[1]s_offset[i]
		if o == 0 {
			v <<= _%[1]s_skips[si] - 1
			si++
			continue
		}
		p0 = p1
		p1 += int(o)
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
//	[2]: 0 a noop
//	[3]: zeroName
//	[4]: 0 a noop
//	[5]: initial value : example "(1)"
//	[6]: cache size limit
const stringBitflagCacheCodeWithSkips = `func (m %[1]s) String() string {
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
	l := len(_%[1]s_offset)
	v := %[1]s(%[5]s)
	si := 0
	p0 := 0
	p1 := 0
	for i := 0; i < l; i, v = i+1, v<<1 {
		o := _%[1]s_offset[i]
		if o == 0 {
			v <<= _%[1]s_skips[si] - 1
			si++
			continue
		}
		p0 = p1
		p1 += int(o)
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
