package logrus

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"runtime"
	"strconv"
	"syscall"
)

const (
	nocolor = 0
	red     = 31
	green   = 32
	yellow  = 33
	blue    = 36
	gray    = 37
)

var (
	baseTimestamp time.Time
	emptyFieldMap FieldMap
)

func init() {
	baseTimestamp = time.Now()
}

// TextFormatter formats logs into text
type TextFormatter struct {
	// Set to true to bypass checking for a TTY before outputting colors.
	ForceColors bool

	// Force disabling colors.
	DisableColors bool

	// Disable timestamp logging. useful when output is redirected to logging
	// system that already adds timestamps.
	DisableTimestamp bool

	// Enable logging the full timestamp when a TTY is attached instead of just
	// the time passed since beginning of execution.
	FullTimestamp bool

	// TimestampFormat to use for display when a full timestamp is printed
	TimestampFormat string

	// The fields are sorted by default for a consistent output. For applications
	// that log extremely frequently and don't use the JSON formatter this may not
	// be desired.
	DisableSorting bool

	// Disables the truncation of the level text to 4 characters.
	DisableLevelTruncation bool

	// QuoteEmptyFields will wrap empty fields in quotes if true
	QuoteEmptyFields bool

	// Whether the logger's out is to a terminal
	isTerminal bool

	// FieldMap allows users to customize the names of keys for default fields.
	// As an example:
	// formatter := &TextFormatter{
	//     FieldMap: FieldMap{
	//         FieldKeyTime:  "@timestamp",
	//         FieldKeyLevel: "@level",
	//         FieldKeyMsg:   "@message"}}
	FieldMap FieldMap

	sync.Once
}

func (f *TextFormatter) init(entry *Entry) {
	if entry.Logger != nil {
		f.isTerminal = checkIfTerminal(entry.Logger.Out)
	}
}

// Format renders a single log entry
func (f *TextFormatter) Format(entry *Entry) ([]byte, error) {
	prefixFieldClashes(entry.Data, f.FieldMap)

	keys := make([]string, 0, len(entry.Data))
	for k := range entry.Data {
		keys = append(keys, k)
	}

	if !f.DisableSorting {
		sort.Strings(keys)
	}

	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	f.Do(func() { f.init(entry) })

	isColored := (f.ForceColors || f.isTerminal) && !f.DisableColors

	timestampFormat := f.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = defaultTimestampFormat
	}
	if isColored {
		f.printColored(b, entry, keys, timestampFormat)
	} else {
		if !f.DisableTimestamp {
			f.appendKeyValue(b, "time", entry.Time.Format(timestampFormat))
		}
		f.appendKeyValue(b, f.FieldMap.resolve(FieldKeyLevel), entry.Level.String())
		f.appendKeyValue(b, "process ID", strconv.Itoa(syscall.Getpid()))
		f.appendKeyValue(b, "thread ID", strconv.Itoa(GetCurrentThreadId()))
		f.appendKeyValue(b, "OS", detectOS())
		
		for _, key := range keys {
			if key == "source_file" {
				n := strings.LastIndexByte(entry.Data[key].(string), '/')
				f.appendKeyValue(b, key, entry.Data[key].(string)[n+1:])
			} else {
				f.appendKeyValue(b, key, entry.Data[key])
			}
		}
		
		if entry.Message != "" {
			f.appendKeyValue(b, f.FieldMap.resolve(FieldKeyMsg), entry.Message)
		}
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

func detectOS() string {
	switch osplatform := runtime.GOOS; osplatform {
	case "windows":
		return "W"
	case "darwin":
		return "M"
	default:
		return "L"
	}

}

func (f *TextFormatter) printColored(b *bytes.Buffer, entry *Entry, keys []string, timestampFormat string) {
	var levelColor int
	switch entry.Level {
	case DebugLevel:
		levelColor = gray
	case WarnLevel:
		levelColor = yellow
	case ErrorLevel, FatalLevel, PanicLevel:
		levelColor = red
	default:
		levelColor = blue
	}

	levelText := strings.ToUpper(entry.Level.String())
	if !f.DisableLevelTruncation {
		levelText = levelText[0:4]
	}

	if f.DisableTimestamp {
		fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m %-44s ", levelColor, levelText, entry.Message)
	} else if !f.FullTimestamp {
		fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%04d] %-44s ", levelColor, levelText, int(entry.Time.Sub(baseTimestamp)/time.Second), entry.Message)
	} else {
		fmt.Fprintf(b, "\x1b[%dm%s\x1b[0m[%s] %-44s ", levelColor, levelText, entry.Time.Format(timestampFormat), entry.Message)
	}
	for _, k := range keys {
		v := entry.Data[k]
		fmt.Fprintf(b, " \x1b[%dm%s\x1b[0m=", levelColor, k)
		f.appendValue(b, v)
	}
}

func (f *TextFormatter) needsQuoting(text string) bool {
	if f.QuoteEmptyFields && len(text) == 0 {
		return true
	}
	for _, ch := range text {
		if !((ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '.' || ch == '_' || ch == '/' || ch == '@' || ch == '^' || ch == '+') {
			return true
		}
	}
	return false
}

func (f *TextFormatter) appendKeyValue(b *bytes.Buffer, key string, value interface{}) {
	switch value := value.(type) {
	case string:
		if key == "time" {
			arrstr := strings.Split(value, "T")
			arr := strings.Split(arrstr[0], "-")
			for i := len(arr) - 1; i >= 0; i-- {
				if i == 0 {
					fmt.Fprintf(b, "%s ", arr[i])
					break
				}
				fmt.Fprintf(b, "%s"+"-", arr[i])
			}
			time := arrstr[1]
			fmt.Fprintf(b, "%s", time[:8])
			break
		} else if key == "level" {
			fmt.Fprintf(b, "[%s]", value)
			break
		} else if key == "process ID" {
			fmt.Fprintf(b, "[pid %s]", value)
			break
		} else if key == "thread ID" {
			fmt.Fprintf(b, "[tid %s]", value)
			break
		} else if key == "OS" {
			fmt.Fprintf(b, "[%s]", value)
			break
		} else if key == "msg" {
			fmt.Fprintf(b, "%s", value)
			break
		} else if key == "source_file" {
			fmt.Fprintf(b, "[%s]", strings.Replace(value, ".go", "", -1))
			break
		}
		fmt.Fprintf(b, "%s", value)
	case error:
		errmsg := value.Error()
		if !f.needsQuoting(errmsg) {
			b.WriteString(errmsg)
		} else {
			fmt.Fprintf(b, "%q", errmsg)
		}
	default:
		fmt.Fprint(b, value)
	}

	b.WriteByte(' ')
}

func (f *TextFormatter) appendValue(b *bytes.Buffer, value interface{}) {
	stringVal, ok := value.(string)
	if !ok {
		stringVal = fmt.Sprint(value)
	}

	if !f.needsQuoting(stringVal) {
		b.WriteString(stringVal)
	} else {
		b.WriteString(fmt.Sprintf("%q", stringVal))
	}
}
