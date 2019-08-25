// Based heavily on https://github.com/inconshreveable/log15/blob/master/format.go

package fmt15

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
)

const (
	DefTimeFmt         = "2006-01-02T15:04:05.000-0700"
	DefFloatFmt        = 'f'
	DefTermMsgJust     = 40
	DefMultilinePrefix = ">  "
	DefColorFmt        = "\x1b[%dm%s\x1b[0m"
	ErrorKey           = "LOG15_ERROR"
)

var (
	TimeFmt         = DefTimeFmt
	FloatFmt        = DefFloatFmt
	TermMsgJust     = DefTermMsgJust
	MultilinePrefix = DefMultilinePrefix
	ColorFmt        = DefColorFmt
)

var ColorMap = map[log15.Lvl]int{
	log15.LvlCrit: 35, log15.LvlError: 31, log15.LvlWarn: 33,
	log15.LvlInfo: 32, log15.LvlDebug: 36,
}

type FCtx []interface{}

func Fmt15Format(colorMap map[log15.Lvl]int) log15.Format {
	return log15.FormatFunc(func(r *log15.Record) []byte {
		color := 0
		if colorMap != nil {
			color = colorMap[r.Lvl]
		}
		buf := &bytes.Buffer{}
		Fmt15(buf, r, color)
		raw := buf.Bytes()
		if raw[len(raw)-1] != '\n' {
			// Add a trailing newline, if the output doesn't already have one.
			raw = append(raw, '\n')
		}
		return raw
	})
}

func Fmt15(buf *bytes.Buffer, r *log15.Record, color int) {
	lvlStr := ColorStr(strings.ToUpper(r.Lvl.String()), color)
	fmt.Fprintf(buf, "%v [%v] %v", r.Time.Format(TimeFmt), lvlStr, r.Msg)

	for i := 0; i < len(r.Ctx); i += 2 {
		k, ok := r.Ctx[i].(string)
		v := FmtValue(r.Ctx[i+1])
		if !ok {
			k, v = ErrorKey, fmt.Sprintf("\"Key(%T) is not a string: %v\"", r.Ctx[i], r.Ctx[i])
		}
		fmt.Fprintf(buf, " %v=%v", ColorStr(k, color), v)
	}
}

func FmtValue(val interface{}) string {
	switch v := val.(type) {
	case time.Time:
		return v.Format(TimeFmt)
	case bool:
		return strconv.FormatBool(v)
	case float32:
		return strconv.FormatFloat(float64(v), byte(FloatFmt), 3, 64)
	case float64:
		return strconv.FormatFloat(v, byte(FloatFmt), 3, 64)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	}
	s := fmt.Sprintf("%+v", val)
	switch {
	case strings.ContainsRune(s, '\n'):
		return FmtMultiLine(s)
	case strings.ContainsAny(s, " \\"):
		return fmt.Sprintf("\"%v\"", s)
	case s == "":
		return "\"\""
	default:
		return s
	}
}

func FmtMultiLine(s string) string {
	var buf bytes.Buffer
	lines := strings.Split(s, "\n")
	buf.WriteByte('\n')
	for i, line := range lines {
		if i == len(lines)-1 && strings.HasSuffix(s, "\n") {
			// Don't output an empty line if caused by a trailing newline in
			// the input.
			break
		}
		fmt.Fprintf(&buf, "%v%v\n", MultilinePrefix, line)
	}
	return buf.String()
}

func ColorStr(s string, color int) string {
	if color == 0 {
		return s
	}
	return fmt.Sprintf(ColorFmt, color, s)
}
