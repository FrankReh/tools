// Copyright 2018 Frank Rehwinkel
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains simple golden tests for various bitflag examples.
// Besides validating the results when the implementation changes,
// it provides a way to look at the generated code without having
// to execute the print statements in one's head.

package main

import (
	"strings"
	"testing"
)

// GoldenBitflag represents a test case for bitflag constants.
type GoldenBitflag struct {
	name            string
	trimPrefix      string
	lineComment     bool
	input           string // input; the package clause is provided when running the test.
	nocache_output  string // exected output having specified nocache to the generator.
	yescache_output string // exected output with the generator allowed to use a map to cache results.
}

var goldenbitflag = []GoldenBitflag{
	{"days", "", false, days_in_bitflag, days_out_bitflag, days_out_bitflag_cache},
	{"gap", "", false, gap_in_bitflag, gap_out_bitflag, gap_out_bitflag_cache},
	{"largegap", "", false, largegap_in_bitflag, largegap_out_bitflag, largegap_out_bitflag_cache},
	{"largestgap", "", false, largestgap_in_bitflag, largestgap_out_bitflag, largestgap_out_bitflag_cache},
}

func TestGoldenBitflag(t *testing.T) {
	for _, test := range goldenbitflag {
		t.Run("nocache"+test.name, func(t *testing.T) {
			g := Generator{
				trimPrefix:  test.trimPrefix,
				lineComment: test.lineComment,
				bitflag:     true,
				cache:       false,
			}
			input := "package test\n" + test.input
			file := test.name + ".go"
			conf := stringerConfig()
			f, err := conf.ParseFile(file, input)
			if err != nil {
				t.Fatal(err)
			}
			conf.CreateFromFiles(test.name, f)
			prog, err := conf.Load()
			if err != nil {
				t.Fatal(err)
			}

			for _, info := range prog.InitialPackages() {
				// Extract the name and type of the constant from the first line.
				tokens := strings.SplitN(test.input, " ", 3)
				if len(tokens) != 3 {
					t.Fatalf("%s: need type declaration on first line", test.name)
				}
				g.generate(info, tokens[1])
				got := string(g.format())
				if got != test.nocache_output {
					t.Errorf("%s: got\n====\n%s====\nexpected\n====%s", test.name, got, test.nocache_output)
				}
			}
		})
		t.Run("yescache"+test.name, func(t *testing.T) {
			g := Generator{
				trimPrefix:  test.trimPrefix,
				lineComment: test.lineComment,
				bitflag:     true,
				cache:       true,
			}
			input := "package test\n" + test.input
			file := test.name + ".go"
			conf := stringerConfig()
			f, err := conf.ParseFile(file, input)
			if err != nil {
				t.Fatal(err)
			}
			conf.CreateFromFiles(test.name, f)
			prog, err := conf.Load()
			if err != nil {
				t.Fatal(err)
			}

			for _, info := range prog.InitialPackages() {
				// Extract the name and type of the constant from the first line.
				tokens := strings.SplitN(test.input, " ", 3)
				if len(tokens) != 3 {
					t.Fatalf("%s: need type declaration on first line", test.name)
				}
				g.generate(info, tokens[1])
				got := string(g.format())
				if got != test.yescache_output {
					t.Errorf("%s: got\n====\n%s====\nexpected\n====%s", test.name, got, test.yescache_output)
				}
			}
		})
	}
}

const days_in_bitflag = `type Days int
const (
	Monday Days = 1 << iota
	Tuesday
	Wednesday
	Thursday
	Friday
	Saturday
	Sunday
)
`

const days_out_bitflag = `
const _Days_name = "MondayTuesdayWednesdayThursdayFridaySaturdaySunday"

var _Days_offset = [...]uint8{6, 7, 9, 8, 6, 8, 6}

func (m Days) String() string {
	if m == 0 {
		return "Days(0)"
	}

	var b []byte
	l := len(_Days_offset)
	v := Days(1)
	p0 := 0
	p1 := 0
	for i := 0; i < l; i, v = i+1, v<<1 {
		p0 = p1
		p1 += int(_Days_offset[i])
		if v&m == 0 {
			continue
		}
		m ^= v
		if len(b) == 0 {
			if m == 0 {
				return _Days_name[p0:p1]
			}
			b = append(b, '(')
		} else {
			b = append(b, '|')
		}
		b = append(b, _Days_name[p0:p1]...)
		if m == 0 {
			b = append(b, ')')
			return string(b)
		}
	}
	s := "Days(0x" + strconv.FormatUint(uint64(m), 16) + ")"
	if len(b) == 0 {
		return s
	}
	b = append(b, '|')
	b = append(b, s...)
	b = append(b, ')')
	return string(b)
}
`

const days_out_bitflag_cache = `
const _Days_name = "MondayTuesdayWednesdayThursdayFridaySaturdaySunday"

var (
	_Days_offset  = [...]uint8{6, 7, 9, 8, 6, 8, 6}
	_Days_cache   = make(map[Days]string)
	_Days_cachemu sync.Mutex
)

func (m Days) String() string {
	_Days_cachemu.Lock()
	s, ok := _Days_cache[m]
	_Days_cachemu.Unlock()
	if ok {
		return s
	}
	s = m._string()
	_Days_cachemu.Lock()
	if len(_Days_cache) >= 256 {
		_Days_cache = make(map[Days]string, 256)
	}
	_Days_cache[m] = s
	_Days_cachemu.Unlock()
	return s
}

func (m Days) _string() string {
	if m == 0 {
		return "Days(0)"
	}

	var b []byte
	l := len(_Days_offset)
	v := Days(1)
	p0 := 0
	p1 := 0
	for i := 0; i < l; i, v = i+1, v<<1 {
		p0 = p1
		p1 += int(_Days_offset[i])
		if v&m == 0 {
			continue
		}
		m ^= v
		if len(b) == 0 {
			if m == 0 {
				return _Days_name[p0:p1]
			}
			b = append(b, '(')
		} else {
			b = append(b, '|')
		}
		b = append(b, _Days_name[p0:p1]...)
		if m == 0 {
			b = append(b, ')')
			return string(b)
		}
	}
	s := "Days(0x" + strconv.FormatUint(uint64(m), 16) + ")"
	if len(b) == 0 {
		return s
	}
	b = append(b, '|')
	b = append(b, s...)
	b = append(b, ')')
	return string(b)
}
`

// Gaps and an offset.
const gap_in_bitflag = `type Gap int
const (
	Zero   Gap = 0
	Two    Gap = 1<< 2
	Three  Gap = 1<< 3
	Five   Gap = 1<< 5
	Six    Gap = 1<< 6
	Seven  Gap = 1<< 7
	Eight  Gap = 1<< 8
	Nine   Gap = 1<< 9
	Eleven Gap = 1<< 11
)
`

const gap_out_bitflag = `
const _Gap_name = "TwoThreeFiveSixSevenEightNineEleven"

var (
	_Gap_offset = [...]uint8{3, 5, 0, 4, 3, 5, 5, 4, 0, 6}
	_Gap_skips  = [...]uint8{1, 1}
)

func (m Gap) String() string {
	if m == 0 {
		return "Zero"
	}

	var b []byte
	l := len(_Gap_offset)
	v := Gap(4)
	si := 0
	p0 := 0
	p1 := 0
	for i := 0; i < l; i, v = i+1, v<<1 {
		o := _Gap_offset[i]
		if o == 0 {
			v <<= _Gap_skips[si] - 1
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
				return _Gap_name[p0:p1]
			}
			b = append(b, '(')
		} else {
			b = append(b, '|')
		}
		b = append(b, _Gap_name[p0:p1]...)
		if m == 0 {
			b = append(b, ')')
			return string(b)
		}
	}
	s := "Gap(0x" + strconv.FormatUint(uint64(m), 16) + ")"
	if len(b) == 0 {
		return s
	}
	b = append(b, '|')
	b = append(b, s...)
	b = append(b, ')')
	return string(b)
}
`

const gap_out_bitflag_cache = `
const _Gap_name = "TwoThreeFiveSixSevenEightNineEleven"

var (
	_Gap_offset  = [...]uint8{3, 5, 0, 4, 3, 5, 5, 4, 0, 6}
	_Gap_skips   = [...]uint8{1, 1}
	_Gap_cache   = make(map[Gap]string)
	_Gap_cachemu sync.Mutex
)

func (m Gap) String() string {
	_Gap_cachemu.Lock()
	s, ok := _Gap_cache[m]
	_Gap_cachemu.Unlock()
	if ok {
		return s
	}
	s = m._string()
	_Gap_cachemu.Lock()
	if len(_Gap_cache) >= 256 {
		_Gap_cache = make(map[Gap]string, 256)
	}
	_Gap_cache[m] = s
	_Gap_cachemu.Unlock()
	return s
}

func (m Gap) _string() string {
	if m == 0 {
		return "Zero"
	}

	var b []byte
	l := len(_Gap_offset)
	v := Gap(4)
	si := 0
	p0 := 0
	p1 := 0
	for i := 0; i < l; i, v = i+1, v<<1 {
		o := _Gap_offset[i]
		if o == 0 {
			v <<= _Gap_skips[si] - 1
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
				return _Gap_name[p0:p1]
			}
			b = append(b, '(')
		} else {
			b = append(b, '|')
		}
		b = append(b, _Gap_name[p0:p1]...)
		if m == 0 {
			b = append(b, ')')
			return string(b)
		}
	}
	s := "Gap(0x" + strconv.FormatUint(uint64(m), 16) + ")"
	if len(b) == 0 {
		return s
	}
	b = append(b, '|')
	b = append(b, s...)
	b = append(b, ')')
	return string(b)
}
`

// Large gap.
const largegap_in_bitflag = `type Gap uint64
const (
	Seven      Gap = 1 << 7
	ThirtyOne  Gap = 1 << 31
	SixtyThree Gap = 1 << 63
)
`

const largegap_out_bitflag = `
const _Gap_name = "SevenThirtyOneSixtyThree"

var (
	_Gap_offset = [...]uint8{5, 0, 9, 0, 10}
	_Gap_skips  = [...]uint8{23, 31}
)

func (m Gap) String() string {
	if m == 0 {
		return "Gap(0)"
	}

	var b []byte
	l := len(_Gap_offset)
	v := Gap(128)
	si := 0
	p0 := 0
	p1 := 0
	for i := 0; i < l; i, v = i+1, v<<1 {
		o := _Gap_offset[i]
		if o == 0 {
			v <<= _Gap_skips[si] - 1
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
				return _Gap_name[p0:p1]
			}
			b = append(b, '(')
		} else {
			b = append(b, '|')
		}
		b = append(b, _Gap_name[p0:p1]...)
		if m == 0 {
			b = append(b, ')')
			return string(b)
		}
	}
	s := "Gap(0x" + strconv.FormatUint(uint64(m), 16) + ")"
	if len(b) == 0 {
		return s
	}
	b = append(b, '|')
	b = append(b, s...)
	b = append(b, ')')
	return string(b)
}
`

const largegap_out_bitflag_cache = `
const _Gap_name = "SevenThirtyOneSixtyThree"

var (
	_Gap_offset  = [...]uint8{5, 0, 9, 0, 10}
	_Gap_skips   = [...]uint8{23, 31}
	_Gap_cache   = make(map[Gap]string)
	_Gap_cachemu sync.Mutex
)

func (m Gap) String() string {
	_Gap_cachemu.Lock()
	s, ok := _Gap_cache[m]
	_Gap_cachemu.Unlock()
	if ok {
		return s
	}
	s = m._string()
	_Gap_cachemu.Lock()
	if len(_Gap_cache) >= 256 {
		_Gap_cache = make(map[Gap]string, 256)
	}
	_Gap_cache[m] = s
	_Gap_cachemu.Unlock()
	return s
}

func (m Gap) _string() string {
	if m == 0 {
		return "Gap(0)"
	}

	var b []byte
	l := len(_Gap_offset)
	v := Gap(128)
	si := 0
	p0 := 0
	p1 := 0
	for i := 0; i < l; i, v = i+1, v<<1 {
		o := _Gap_offset[i]
		if o == 0 {
			v <<= _Gap_skips[si] - 1
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
				return _Gap_name[p0:p1]
			}
			b = append(b, '(')
		} else {
			b = append(b, '|')
		}
		b = append(b, _Gap_name[p0:p1]...)
		if m == 0 {
			b = append(b, ')')
			return string(b)
		}
	}
	s := "Gap(0x" + strconv.FormatUint(uint64(m), 16) + ")"
	if len(b) == 0 {
		return s
	}
	b = append(b, '|')
	b = append(b, s...)
	b = append(b, ')')
	return string(b)
}
`

// Largest gap.
const largestgap_in_bitflag = `type Gap uint64
const (
	Zero       Gap = 1 << 0
	SixtyThree Gap = 1 << 63
)
`

const largestgap_out_bitflag = `
const _Gap_name = "ZeroSixtyThree"

var (
	_Gap_offset = [...]uint8{4, 0, 10}
	_Gap_skips  = [...]uint8{62}
)

func (m Gap) String() string {
	if m == 0 {
		return "Gap(0)"
	}

	var b []byte
	l := len(_Gap_offset)
	v := Gap(1)
	si := 0
	p0 := 0
	p1 := 0
	for i := 0; i < l; i, v = i+1, v<<1 {
		o := _Gap_offset[i]
		if o == 0 {
			v <<= _Gap_skips[si] - 1
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
				return _Gap_name[p0:p1]
			}
			b = append(b, '(')
		} else {
			b = append(b, '|')
		}
		b = append(b, _Gap_name[p0:p1]...)
		if m == 0 {
			b = append(b, ')')
			return string(b)
		}
	}
	s := "Gap(0x" + strconv.FormatUint(uint64(m), 16) + ")"
	if len(b) == 0 {
		return s
	}
	b = append(b, '|')
	b = append(b, s...)
	b = append(b, ')')
	return string(b)
}
`

const largestgap_out_bitflag_cache = `
const _Gap_name = "ZeroSixtyThree"

var (
	_Gap_offset  = [...]uint8{4, 0, 10}
	_Gap_skips   = [...]uint8{62}
	_Gap_cache   = make(map[Gap]string)
	_Gap_cachemu sync.Mutex
)

func (m Gap) String() string {
	_Gap_cachemu.Lock()
	s, ok := _Gap_cache[m]
	_Gap_cachemu.Unlock()
	if ok {
		return s
	}
	s = m._string()
	_Gap_cachemu.Lock()
	if len(_Gap_cache) >= 256 {
		_Gap_cache = make(map[Gap]string, 256)
	}
	_Gap_cache[m] = s
	_Gap_cachemu.Unlock()
	return s
}

func (m Gap) _string() string {
	if m == 0 {
		return "Gap(0)"
	}

	var b []byte
	l := len(_Gap_offset)
	v := Gap(1)
	si := 0
	p0 := 0
	p1 := 0
	for i := 0; i < l; i, v = i+1, v<<1 {
		o := _Gap_offset[i]
		if o == 0 {
			v <<= _Gap_skips[si] - 1
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
				return _Gap_name[p0:p1]
			}
			b = append(b, '(')
		} else {
			b = append(b, '|')
		}
		b = append(b, _Gap_name[p0:p1]...)
		if m == 0 {
			b = append(b, ')')
			return string(b)
		}
	}
	s := "Gap(0x" + strconv.FormatUint(uint64(m), 16) + ")"
	if len(b) == 0 {
		return s
	}
	b = append(b, '|')
	b = append(b, s...)
	b = append(b, ')')
	return string(b)
}
`
