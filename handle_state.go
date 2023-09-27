// copy from offical
package logger

import (
	"encoding"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

type handleState struct {
	h       *custom_handler
	buf     *Buffer
	freeBuf bool      // should buf be freed?
	sep     string    // separator to write before next key
	prefix  *Buffer   // for text: key prefix
	groups  *[]string // pool-allocated slice of active groups, for ReplaceAttr
}

var groupPool = sync.Pool{New: func() any {
	s := make([]string, 0, 10)
	return &s
}}

func (this *custom_handler) newHandleState(buf *Buffer, freeBuf bool, sep string) handleState {
	s := handleState{
		h:       this,
		buf:     buf,
		freeBuf: freeBuf,
		sep:     sep,
		prefix:  NewBuffer(),
	}
	if this.opts.ReplaceAttr != nil {
		s.groups = groupPool.Get().(*[]string)
		// *s.groups = append(*s.groups, this.groups[:this.nOpenGroups]...)
	}
	return s
}

func (s *handleState) free() {
	if s.freeBuf {
		s.buf.Free()
	}
	if gs := s.groups; gs != nil {
		*gs = (*gs)[:0]
		groupPool.Put(gs)
	}
	s.prefix.Free()
}

func (s *handleState) openGroups() {
	// for _, n := range s.h.groups[s.h.nOpenGroups:] {
	// 	s.openGroup(n)
	// }
}

// Separator for group names and keys.
const keyComponentSep = '.'

// openGroup starts a new group of attributes
// with the given name.
func (s *handleState) openGroup(name string) {
	s.prefix.WriteString(name)
	s.prefix.WriteByte(keyComponentSep)
	if s.groups != nil {
		*s.groups = append(*s.groups, name)
	}
}

// closeGroup ends the group with the given name.
func (s *handleState) closeGroup(name string) {
	(*s.prefix) = (*s.prefix)[:len(*s.prefix)-len(name)-1 /* for keyComponentSep */]
	s.sep = s.h.attrSep()
	if s.groups != nil {
		*s.groups = (*s.groups)[:len(*s.groups)-1]
	}
}

func IsEmpty(a slog.Attr) bool {
	return a.Key == "" && a.Value.Any() == nil
}

// appendAttr appends the Attr's key and value using app.
// It handles replacement and checking for an empty key.
// after replacement).
func (s *handleState) appendAttr(a slog.Attr) {
	if rep := s.h.opts.ReplaceAttr; rep != nil && a.Value.Kind() != slog.KindGroup {
		var gs []string
		if s.groups != nil {
			gs = *s.groups
		}
		// Resolve before calling ReplaceAttr, so the user doesn't have to.
		a.Value = a.Value.Resolve()
		a = rep(gs, a)
	}
	a.Value = a.Value.Resolve()
	// Elide empty Attrs.
	if IsEmpty(a) {
		return
	}
	// Special case: Source.
	if v := a.Value; v.Kind() == slog.KindAny {
		if src, ok := v.Any().(*slog.Source); ok {
			a.Value = slog.StringValue(fmt.Sprintf("%s:%d", src.File, src.Line))
		}
	}
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		// Output only non-empty groups.
		if len(attrs) > 0 {
			// Inline a group with an empty key.
			if a.Key != "" {
				s.openGroup(a.Key)
			}
			for _, aa := range attrs {
				s.appendAttr(aa)
			}
			if a.Key != "" {
				s.closeGroup(a.Key)
			}
		}
	} else {
		if a.Key == slog.TimeKey || a.Key == slog.SourceKey || a.Key == slog.LevelKey {
			s.buf.WriteString(a.Value.String())
			s.buf.WriteString(s.h.attrSep())
		} else {
			s.appendKey(a.Key)
			s.appendValue(a.Value)
		}
	}
}

func (s *handleState) appendError(err error) {
	s.appendString(fmt.Sprintf("!ERROR:%v", err))
}

func (s *handleState) appendKey(key string) {
	s.buf.WriteString(s.sep)
	if s.prefix != nil && len(*s.prefix) > 0 {
		// TODO: optimize by avoiding allocation.
		s.appendString(string(*s.prefix) + key)
	} else {
		s.appendString(key)
	}
	s.buf.WriteByte('=')
	s.sep = s.h.attrSep()
}

func needsQuoting(s string) bool {
	if len(s) == 0 {
		return true
	}
	for i := 0; i < len(s); {
		b := s[i]
		if b < utf8.RuneSelf {
			// Quote anything except a backslash that would need quoting in a
			// JSON string, as well as space and '='
			if b != '\\' && (b == ' ' || b == '=' || !safeSet[b]) {
				return true
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError || unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return true
		}
		i += size
	}
	return false
}

func (s *handleState) appendString(str string) {
	if needsQuoting(str) {
		*s.buf = strconv.AppendQuote(*s.buf, str)
	} else {
		s.buf.WriteString(str)
	}
}

func (s *handleState) appendValue(v slog.Value) {
	var err error
	err = appendTextValue(s, v)
	if err != nil {
		s.appendError(err)
	}
}

func appendTextValue(s *handleState, v slog.Value) error {
	switch v.Kind() {
	case slog.KindString:
		s.appendString(v.String())
	case slog.KindTime:
		s.appendTime(v.Time())
	case slog.KindAny:
		if tm, ok := v.Any().(encoding.TextMarshaler); ok {
			data, err := tm.MarshalText()
			if err != nil {
				return err
			}
			// TODO: avoid the conversion to string.
			s.appendString(string(data))
			return nil
		}
		if bs, ok := byteSlice(v.Any()); ok {
			// As of Go 1.19, this only allocates for strings longer than 32 bytes.
			s.buf.WriteString(strconv.Quote(string(bs)))
			return nil
		}
		s.appendString(fmt.Sprintf("%+v", v.Any()))
	default:
		*s.buf = AppendSlogValue(v, *s.buf)
	}
	return nil
}

func AppendSlogValue(v slog.Value, dst []byte) []byte {
	switch v.Kind() {
	case slog.KindString:
		return append(dst, v.String()...)
	case slog.KindInt64:
		return strconv.AppendInt(dst, v.Int64(), 10)
	case slog.KindUint64:
		return strconv.AppendUint(dst, v.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.AppendFloat(dst, v.Float64(), 'g', -1, 64)
	case slog.KindBool:
		return strconv.AppendBool(dst, v.Bool())
	case slog.KindDuration:
		return append(dst, v.Duration().String()...)
	case slog.KindTime:
		return append(dst, v.Time().String()...)
	case slog.KindGroup:
		return fmt.Append(dst, v.Group())
	case slog.KindAny, slog.KindLogValuer:
		return fmt.Append(dst, v.Any())
	default:
		panic(fmt.Sprintf("bad kind: %s", v.Kind()))
	}
}

func byteSlice(a any) ([]byte, bool) {
	if bs, ok := a.([]byte); ok {
		return bs, true
	}
	// Like Printf's %s, we allow both the slice type and the byte element type to be named.
	t := reflect.TypeOf(a)
	if t != nil && t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
		return reflect.ValueOf(a).Bytes(), true
	}
	return nil, false
}

func (s *handleState) appendTime(t time.Time) {
	writeTimeRFC3339Millis(s.buf, t)
}

// This takes half the time of Time.AppendFormat.
func writeTimeRFC3339Millis(buf *Buffer, t time.Time) {
	year, month, day := t.Date()
	buf.WritePosIntWidth(year, 4)
	buf.WriteByte('-')
	buf.WritePosIntWidth(int(month), 2)
	buf.WriteByte('-')
	buf.WritePosIntWidth(day, 2)
	buf.WriteByte('T')
	hour, min, sec := t.Clock()
	buf.WritePosIntWidth(hour, 2)
	buf.WriteByte(':')
	buf.WritePosIntWidth(min, 2)
	buf.WriteByte(':')
	buf.WritePosIntWidth(sec, 2)
	ns := t.Nanosecond()
	buf.WriteByte('.')
	buf.WritePosIntWidth(ns/1e6, 3)
	_, offsetSeconds := t.Zone()
	if offsetSeconds == 0 {
		buf.WriteByte('Z')
	} else {
		offsetMinutes := offsetSeconds / 60
		if offsetMinutes < 0 {
			buf.WriteByte('-')
			offsetMinutes = -offsetMinutes
		} else {
			buf.WriteByte('+')
		}
		buf.WritePosIntWidth(offsetMinutes/60, 2)
		buf.WriteByte(':')
		buf.WritePosIntWidth(offsetMinutes%60, 2)
	}
}

func (s *handleState) appendNonBuiltIns(r slog.Record) {
	// preformatted Attrs
	if len(s.h.preformattedAttrs) > 0 {
		s.buf.WriteString(s.sep)
		s.buf.Write(s.h.preformattedAttrs)
		s.sep = s.h.attrSep()
	}
	if r.NumAttrs() > 0 {
		s.prefix.WriteString(s.h.groupPrefix)
		s.openGroups()
		r.Attrs(func(a slog.Attr) bool {
			s.appendAttr(a)
			return true
		})
	}
}

var safeSet = [utf8.RuneSelf]bool{
	' ':      true,
	'!':      true,
	'"':      false,
	'#':      true,
	'$':      true,
	'%':      true,
	'&':      true,
	'\'':     true,
	'(':      true,
	')':      true,
	'*':      true,
	'+':      true,
	',':      true,
	'-':      true,
	'.':      true,
	'/':      true,
	'0':      true,
	'1':      true,
	'2':      true,
	'3':      true,
	'4':      true,
	'5':      true,
	'6':      true,
	'7':      true,
	'8':      true,
	'9':      true,
	':':      true,
	';':      true,
	'<':      true,
	'=':      true,
	'>':      true,
	'?':      true,
	'@':      true,
	'A':      true,
	'B':      true,
	'C':      true,
	'D':      true,
	'E':      true,
	'F':      true,
	'G':      true,
	'H':      true,
	'I':      true,
	'J':      true,
	'K':      true,
	'L':      true,
	'M':      true,
	'N':      true,
	'O':      true,
	'P':      true,
	'Q':      true,
	'R':      true,
	'S':      true,
	'T':      true,
	'U':      true,
	'V':      true,
	'W':      true,
	'X':      true,
	'Y':      true,
	'Z':      true,
	'[':      true,
	'\\':     false,
	']':      true,
	'^':      true,
	'_':      true,
	'`':      true,
	'a':      true,
	'b':      true,
	'c':      true,
	'd':      true,
	'e':      true,
	'f':      true,
	'g':      true,
	'h':      true,
	'i':      true,
	'j':      true,
	'k':      true,
	'l':      true,
	'm':      true,
	'n':      true,
	'o':      true,
	'p':      true,
	'q':      true,
	'r':      true,
	's':      true,
	't':      true,
	'u':      true,
	'v':      true,
	'w':      true,
	'x':      true,
	'y':      true,
	'z':      true,
	'{':      true,
	'|':      true,
	'}':      true,
	'~':      true,
	'\u007f': true,
}
