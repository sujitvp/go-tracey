# go-tracey

[![Build Status](https://travis-ci.org/sujitvp/go-tracey.svg?branch=master)](https://travis-ci.org/sujitvp/go-tracey) [![Coverage Status](https://coveralls.io/repos/sujitvp/go-tracey/badge.svg?branch=master)](https://coveralls.io/r/sujitvp/go-tracey?branch=master)

A simple function tracer for Go, with support for concurrency (multi-thread) separation

## Install

```sh
go get github.com/sujitvp/go-tracey
```

## Basic Usage

*file: foo.go*
```go
package main

import (
    "github.com/sujitvp/go-tracey"
)

// Setup global trace function (default options)
var Trace = tracey.New(nil)

//Example 1 - call without args => generates function name  in output
func bar() {
    defer Trace()()
    foo(2)
}

//Example 2 - call with function arguments => generates function name & arguments
func foo(i int) {
    // $FN will get replaced with the function's name
    defer Trace("$FN(%d)", i)()
    if i != 0 {
        foo(i - 1)
    }
}

func main() {
    defer Trace()()
    bar(2)
 	// Example 3 - Tracing unnamed/internal functions & logging args
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        defer Trace("http.Redirect(%s, %s, %s, %d)", w, r, "/newpath", http.StatusMovedPermanently)()
		http.Redirect(w, r, "/newpath", http.StatusMovedPermanently)
	})

 	// Example 4 - Tracing unnamed/internal functions run concurrently
    go func() {
		defer Trace("Http listener")()
		if err := http.ListenAndServe(":8000", nil); err != nil {
			log.Fatalf("cannot start Web server: %s", err)
		}
	}()
}
```

```sh
go run foo.go 
[ 0]ENTER: main
[ 1]  ENTER: foo(2)
[ 2]    ENTER: foo(1)
[ 3]      ENTER: foo(0)
[ 3]      EXIT:  foo(0)
[ 2]    EXIT:  foo(1)
[ 1]  EXIT:  foo(2)
[ 0]EXIT:  main
```

## Configurable Tracey Options

```go
// These options represent the various settings which tracey exposes.
// A pointer to this structure is expected to be passed into the
// `tracey.New(...)` function below.
type Options struct {

	// Setting "DisableTracing" to "true" will cause tracey to return
	// no-op'd functions for both exit() and enter(). The default value
	// for this is "false" which enables tracing.
	DisableTracing bool

	// Setting the "CustomLogger" to nil will cause tracey to log to
	// os.Stdout. Otherwise, this is a pointer to an object as returned
	// from `log.New(...)`.
	CustomLogger *log.Logger

	// Setting "DisableDepthValue" to "true" will cause tracey to not
	// prepend the printed function's depth to enter() and exit() messages.
	// The default value is "false", which logs the depth value.
	DisableDepthValue bool

	// Setting "DisableNesting" to "true" will cause tracey to not indent
	// any messages from nested functions. The default value is "false"
	// which enables nesting by prepending "SpacesPerIndent" number of
	// spaces per level nested.
	DisableNesting  bool
	SpacesPerIndent int `default:"2"`

	// Setting "EnterMessage" or "ExitMessage" will override the default
	// value of "Enter: " and "EXIT:  " respectively.
	EnterMessage string `default:"ENTER: "`
	ExitMessage  string `default:"EXIT:  "`

	// Enables per-method execution time instrumentation
	EnableInstrumentation bool
}
```

## Advanced Usage

Tracey's `Enter()` receives a variadic list interfaces: `...interface{}`. This allows us to pass in a variable number of types. However, the first of such is expected to be a format string, otherwise the function just logs the function's name. If a format string is specified with a `$FN` token, then said token is replaced for the actual function's name.

```go
var Trace = tracey.New(nil)
// var t = trace.New(nil)

func Foo() {
    defer Trace("$FN is awesome %d %s", 3, "four")()
```
Will produce: `Foo is awesome 3 four` when `Foo()` is logged.

### Anonymous Functions:

Non-named functions are given a generic name of "func.N" where N is the N-th unnamed function in a given file. If we wish to log these explicitly, we can just give them a suitable name using the format string. For instance:

```go
var Trace = tracey.New(nil)
// var t = trace.New(nil)

func main() {
    defer Trace()()
    func() {
        defer Trace("InnerFunction")()
    }()
}
```
Will produce:
```sh
[ 0]ENTER: main
  [ 1]ENTER: InnerFunction
  [ 1]EXIT : InnerFunction
[ 0]EXIT : main
```

## Custom Logger

Logging to a file:
```go
func main() {
	f, _ := os.Create("somelogfile")
	Trace = tracey.New(&tracey.Options{
		DisableTracing: !ftrace,
		CustomLogger:   log.New(f, "", 0),
	})

    defer Trace()()
}

```
Will write out the Trace to the log file.

By using the same io.writer for multiple loggers, it is possible to combine the logs (including Tracey's trace logs) into a single log file.

The `tracey_test.go` file also provides an example of creating a custom logger out of a `[]byte` so we can compare whats written out to what we expect to output.

## Want to help out?

I appreciate any and all feedback. There is no "real" coding standard. I am still finding my feet in Go and am not sure what I like and abhor yet.

All pull requests and commits will be run against Travis-CI, and results will be forwarded to Coveralls.io.

Here is the CI page for the project: [![Build Status](https://travis-ci.org/sabhiram/go-tracey.svg?branch=master)](https://travis-ci.org/sabhiram/go-tracey)

And coverage here: [![Coverage Status](https://coveralls.io/repos/sabhiram/go-tracey/badge.svg?branch=master)](https://coveralls.io/r/sabhiram/go-tracey?branch=master)

To run tests:
```sh
$cd go-tracey
$go test -v
```

[Link to tests for the lazy](https://github.com/sabhiram/go-tracey/blob/master/tracey_test.go)
