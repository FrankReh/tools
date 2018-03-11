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

func TestGoldenBitflag(t *testing.T) {
	for _, test := range []struct {
		name        string
		trimPrefix  string
		lineComment bool
		input       string // input; the package clause is provided when running the test.

		// Two outputs when table format is false
		nocache_notable_output  string // expected output having specified nocache to the generator.
		yescache_notable_output string // expected output with the generator allowed to use a map to cache results.

		// Two outputs when table format is true
		nocache_yestable_output  string
		yescache_yestable_output string
	}{
		{"days", "", false, days_in_bitflag,
			days_out_bitflag, days_out_bitflag_cache,
			days_out_bitflag_table, days_out_bitflag_cache_table},
		{"gap", "", false, gap_in_bitflag,
			gap_out_bitflag, gap_out_bitflag_cache,
			gap_out_bitflag_table, gap_out_bitflag_cache_table},
		{"largegap", "", false, largegap_in_bitflag,
			largegap_out_bitflag, largegap_out_bitflag_cache,
			largegap_out_bitflag_table, largegap_out_bitflag_cache_table},
		{"largestgap", "", false, largestgap_in_bitflag,
			largestgap_out_bitflag, largestgap_out_bitflag_cache,
			largestgap_out_bitflag_table, largestgap_out_bitflag_cache_table},
	} {

		// Run two versions of test, one with cache false, other with cache true.

		for _, table := range []bool{false, true} {
			for _, cache := range []bool{false, true} {
				var cname, expected string
				if table {
					if cache {
						cname = "cache=y,table=y"
						expected = test.yescache_yestable_output
					} else {
						cname = "cache=n,table=y"
						expected = test.nocache_yestable_output
					}
				} else {
					if cache {
						cname = "cache=y,table=n"
						expected = test.yescache_notable_output
					} else {
						cname = "cache=n,table=n"
						expected = test.nocache_notable_output
					}
				}
				expected = string(formatBytes([]byte(expected)))
				t.Run(cname+test.name, func(t *testing.T) {
					g := Generator{
						trimPrefix:  test.trimPrefix,
						lineComment: test.lineComment,
						bitflag:     true,
						cache:       cache,
						table:       table,
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
						if got != expected {
							t.Errorf("%s: got\n====\nlen %d====\nexpected\n====len %d",
								test.name, len(got), len(expected))
							t.Errorf("%s: got\n====\n%s====\nexpected\n====%s",
								test.name, got, expected)
						}
					}
				})
			}
		}
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
	_Days_cachemu sync.RWMutex
)

func (m Days) String() string {
	_Days_cachemu.RLock()
	s, ok := _Days_cache[m]
	_Days_cachemu.RUnlock()
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
const days_out_bitflag_table = `
var _Days_stringer = _stringerBitflag{
	typename: "Days",
	zero:     "Days(0)",
	first:    uint64(1),
	names:    "MondayTuesdayWednesdayThursdayFridaySaturdaySunday",
	offsets:  []uint8{6, 7, 9, 8, 6, 8, 6},
}

func (m Days) String() string {
	return _Days_stringer.mstring(uint64(m))
}
`
const days_out_bitflag_cache_table = `
var _Days_stringer = _stringerBitflagCache{
	sb: _stringerBitflag{
		typename: "Days",
		zero:     "Days(0)",
		first:    uint64(1),
		names:    "MondayTuesdayWednesdayThursdayFridaySaturdaySunday",
		offsets:  []uint8{6, 7, 9, 8, 6, 8, 6},
	},
}

func (m Days) String() string {
	return _Days_stringer.mstring(uint64(m))
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
	_Gap_cachemu sync.RWMutex
)

func (m Gap) String() string {
	_Gap_cachemu.RLock()
	s, ok := _Gap_cache[m]
	_Gap_cachemu.RUnlock()
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
const gap_out_bitflag_table = `
var _Gap_stringer = _stringerBitflag{
	typename: "Gap",
	zero:     "Zero",
	first:    uint64(4),
	names:    "TwoThreeFiveSixSevenEightNineEleven",
	offsets:  []uint8{3, 5, 0, 4, 3, 5, 5, 4, 0, 6},
	skips:    []uint8{1, 1},
}

func (m Gap) String() string {
	return _Gap_stringer.mstring(uint64(m))
}
`
const gap_out_bitflag_cache_table = `
var _Gap_stringer = _stringerBitflagCache{
	sb: _stringerBitflag{
		typename: "Gap",
		zero:     "Zero",
		first:    uint64(4),
		names:    "TwoThreeFiveSixSevenEightNineEleven",
		offsets:  []uint8{3, 5, 0, 4, 3, 5, 5, 4, 0, 6},
		skips:    []uint8{1, 1},
	},
}

func (m Gap) String() string {
	return _Gap_stringer.mstring(uint64(m))
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
	_Gap_cachemu sync.RWMutex
)

func (m Gap) String() string {
	_Gap_cachemu.RLock()
	s, ok := _Gap_cache[m]
	_Gap_cachemu.RUnlock()
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
const largegap_out_bitflag_table = `
var _Gap_stringer = _stringerBitflag{
	typename: "Gap",
	zero:     "Gap(0)",
	first:    uint64(128),
	names:    "SevenThirtyOneSixtyThree",
	offsets:  []uint8{5, 0, 9, 0, 10},
	skips:    []uint8{23, 31},
}

func (m Gap) String() string {
	return _Gap_stringer.mstring(uint64(m))
}
`
const largegap_out_bitflag_cache_table = `
var _Gap_stringer = _stringerBitflagCache{
	sb: _stringerBitflag{
		typename: "Gap",
		zero:     "Gap(0)",
		first:    uint64(128),
		names:    "SevenThirtyOneSixtyThree",
		offsets:  []uint8{5, 0, 9, 0, 10},
		skips:    []uint8{23, 31},
	},
}

func (m Gap) String() string {
	return _Gap_stringer.mstring(uint64(m))
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
	_Gap_cachemu sync.RWMutex
)

func (m Gap) String() string {
	_Gap_cachemu.RLock()
	s, ok := _Gap_cache[m]
	_Gap_cachemu.RUnlock()
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
const largestgap_out_bitflag_table = `
var _Gap_stringer = _stringerBitflag{
	typename: "Gap",
	zero:     "Gap(0)",
	first:    uint64(1),
	names:    "ZeroSixtyThree",
	offsets:  []uint8{4, 0, 10},
	skips:    []uint8{62},
}

func (m Gap) String() string {
	return _Gap_stringer.mstring(uint64(m))
}
`
const largestgap_out_bitflag_cache_table = `
var _Gap_stringer = _stringerBitflagCache{
	sb: _stringerBitflag{
		typename: "Gap",
		zero:     "Gap(0)",
		first:    uint64(1),
		names:    "ZeroSixtyThree",
		offsets:  []uint8{4, 0, 10},
		skips:    []uint8{62},
	},
}

func (m Gap) String() string {
	return _Gap_stringer.mstring(uint64(m))
}
`
