// `Tracey` is a simple library which allows for much easier function enter / exit logging
package tracey

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"reflect"
	"runtime"
	"sync"
	"time"
)

// Define a global regex for extracting function names
var RE_stripFnPreamble = regexp.MustCompile(`^.*\/(.*)$`)
var RE_detectFN = regexp.MustCompile(`\$FN`)

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

// Private member, used to keep track of how many levels of nesting
// the current trace functions have navigated.
var currentDepth struct {
	sync.RWMutex
	d map[uint64]int
}
var entryTime struct {
	sync.RWMutex
	t map[string]time.Time
}

// New is the main entry-point for the tracey lib. Calling New with nil will
// result in the default options being used.
func New(opts *Options) func(...interface{}) func() {
	var options Options
	if opts != nil {
		options = *opts
	}

	// If tracing is not enabled, just return no-op functions
	if options.DisableTracing {
		return func(s ...interface{}) func() { return func() {} }
	}

	// Revert to stdout if no logger is defined
	if options.CustomLogger == nil {
		options.CustomLogger = log.New(os.Stdout, "", 0)
	}

	// Use reflect to deduce "default" values for the
	// Enter and Exit messages (if they are not set)
	reflectedType := reflect.TypeOf(options)
	if options.EnterMessage == "" {
		field, _ := reflectedType.FieldByName("EnterMessage")
		options.EnterMessage = field.Tag.Get("default")
	}
	if options.ExitMessage == "" {
		field, _ := reflectedType.FieldByName("ExitMessage")
		options.ExitMessage = field.Tag.Get("default")
	}

	// If nesting is enabled, and the spaces are not specified,
	// use the "default" value
	if options.DisableNesting {
		options.SpacesPerIndent = 0
	} else {
		currentDepth.d = make(map[uint64]int, 20)
		if options.SpacesPerIndent == 0 {
			field, _ := reflectedType.FieldByName("SpacesPerIndent")
			options.SpacesPerIndent, _ = strconv.Atoi(field.Tag.Get("default"))
		}
	}

	_getGID := func() uint64 {
		b := make([]byte, 64)
		b = b[:runtime.Stack(b, false)]
		b = bytes.TrimPrefix(b, []byte("goroutine "))
		b = b[:bytes.IndexByte(b, ' ')]
		n, _ := strconv.ParseUint(string(b), 10, 64)
		return n
	}

	if options.EnableInstrumentation {
		entryTime.t = make(map[string]time.Time, 20)
	}

	//
	// Define functions we will use and return to the caller
	//
	_spacify := func() string {
		var spaces string
		if !options.DisableNesting {
			currentDepth.RLock()
			d := currentDepth.d[_getGID()]
			currentDepth.RUnlock()
			spaces = strings.Repeat(" ", d*options.SpacesPerIndent)
			if !options.DisableDepthValue {
				return fmt.Sprintf("[%2d]%s", d, spaces)
			}
		}
		return spaces
	}

	// Increment function to increase the current depth value
	_incrementDepth := func() {
		if !options.DisableNesting {
			currentDepth.Lock()
			currentDepth.d[_getGID()]++
			currentDepth.Unlock()
		}
	}

	// Decrement function to decrement the current depth value
	//  + panics if current depth value is < 0
	_decrementDepth := func() {
		if !options.DisableNesting {
			gid := _getGID()
			currentDepth.Lock()
			currentDepth.d[gid]--
			if currentDepth.d[gid] < 0 {
				//panic("Depth is negative! Should never happen!")
				//panic in function tracing does not make sense
				// instead reset the depth, and log warning
				options.CustomLogger.Println("Warning: depth became negative in tracey, when attempting to decrement.")
				currentDepth.d[gid] = 0
			}
			currentDepth.Unlock()
		}
	}

	_getname := func(s ...interface{}) string {
		// Figure out the name of the caller and use that
		fnName := "<unknown>"
		pc, fl, fi, ok := runtime.Caller(2)
		if ok {
			fnName = RE_stripFnPreamble.ReplaceAllString(runtime.FuncForPC(pc).Name(), "$1")
			//fnName = runtime.FuncForPC(pc).Name()
		}

		if fnName == "" {
			fnName = fl + strconv.Itoa(fi)
		}
		//		if len(args) > 0 {
		//			if fmtStr, ok := args[0].(string); ok {
		//				// We have a string leading args, assume its to be formatted
		//				traceMessage = fmt.Sprintf(fmtStr, args[1:]...)
		//			}
		//		}

		// "$FN" will be replaced by the name of the function (if present)
		//		traceMessage = RE_detectFN.ReplaceAllString(traceMessage, fnName)
		tid := "[tid:" + strconv.FormatUint(_getGID(), 10)
		var traceMessage string
		if len(s) > 0 {
			fmtStr, ok := s[0].(string)
			if len(s) == 1 && ok {
				tid = tid + " - " + s[0].(string)
			} else if ok {
				// We have a string leading args, assume its to be formatted
				traceMessage = fmt.Sprintf(fmtStr, s[1:]...)
			}
		}

		fnName = tid + "]=>" + RE_detectFN.ReplaceAllString(traceMessage, fnName)
		return fnName
	}

	//	_instrument := func() uint64 {
	//		return 0
	//	}

	// Exit function, invoked on function exit (usually deferred)
	_exit := func() {
		_decrementDepth()
		fname := _getname()
		if options.EnableInstrumentation {
			entryTime.RLock()
			fname = fname + " ... in " + time.Since(entryTime.t[fname]).String()
			entryTime.RUnlock()
		}
		options.CustomLogger.Printf("%s%s%s\n", _spacify(), options.ExitMessage, fname)
		if options.EnableInstrumentation {
			entryTime.Lock()
			delete(entryTime.t, fname)
			entryTime.Unlock()
		}
	}

	// Enter function, invoked on function entry
	_enter := func(s ...interface{}) func() {
		defer _incrementDepth()

		fname := _getname(s...)
		if options.EnableInstrumentation {
			entryTime.Lock()
			entryTime.t[fname] = time.Now()
			entryTime.Unlock()
		}
		options.CustomLogger.Printf("%s%s%s\n", _spacify(), options.EnterMessage, fname)
		//		return traceMessage
		return _exit
	}

	return _enter
}
