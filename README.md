# Go Tools: guru and stringer

This repository is forked from https://github.com/golang/tools which itself is a
mirror of https://golang.org/x/tools

Two commands are modified in this fork: guru and stringer.

Guru is modified to allow it to work with cgo files.

Stringer is modified to allow it to also work with cgo files.
In addition, stringer is modified to handle bit pattern constants.

## Download/Install

Because the original import paths remain in
the code, the guru and stringer commands build only after also
installing golang.org/x/tools. Refer to https://golang.org/x/tools.

After the two repos are downloaded, test and build the two commands
from their respective directories, `cmd/guru` and `cmd/stringer`.

## License

The license is unchanged from the forked version. It is the BSD 3-clause license.

## Stringer bitflag

Only single-bit bitflag constants are supported. Constants with multiple bits
set are silently ignored.

A small example:

```
type M int

//go:generate stringer -bitflag -type=M

const (
	A M = 1 << iota
	B
)

fmt.Println(A)     // prints "A"
fmt.Println(B | A) // prints "(A|B)"
```
