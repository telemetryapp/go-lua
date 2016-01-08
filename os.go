package lua

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"
)

func field(l *State, key string, def int) int {
	l.Field(-1, key)
	r, ok := l.ToInteger(-1)
	if !ok {
		if def < 0 {
			Errorf(l, "field '%s' missing in date table", key)
		}
		r = def
	}
	l.Pop(1)
	return r
}

// http://www.lua.org/pil/22.1.html
var conversion = map[rune]string{
	/*stdWeekDay        */ 'a': "Mon",
	/*stdLongWeekDay    */ 'A': "Monday",
	/*stdMonth          */ 'b': "Jan",
	/*stdLongMonth      */ 'B': "January",
	/*                  */ 'c': "01/02/06 15:04:05",
	/*stdZeroDay        */ 'd': "02",
	/*stdHour           */ 'H': "15",
	/*stdZeroHour12     */ 'I': "03",
	/*stdZeroMinute     */ 'M': "04",
	/*stdZeroMonth      */ 'm': "01",
	/*stdPM             */ 'p': "pm",
	/*stdZeroSecond     */ 'S': "05",
	/* @todo %w	weekday (3) [0-6 = Sunday-Saturday]	*/
	/*                  */ 'x': "01/02/06",
	/*                  */ 'X': "15:04:05",
	/*stdLongYear       */ 'Y': "2006",
	/*stdYear           */ 'y': "06",
	/*                  */ '%': "%",
}

// Taken from https://github.com/jehiah/go-strftime
// This is an alternative to time.Format because no one knows
// what date 040305 is supposed to create when used as a 'layout' string
// this takes standard strftime format options. For a complete list
// of format options see http://strftime.org/
func Format(format string, t time.Time) string {
	retval := make([]byte, 0, len(format))
	for i, ni := 0, 0; i < len(format); i = ni + 2 {
		ni = strings.IndexByte(format[i:], '%')
		if ni < 0 {
			ni = len(format)
		} else {
			ni += i
		}
		retval = append(retval, []byte(format[i:ni])...)
		if ni+1 < len(format) {
			c := format[ni+1]
			if c == '%' {
				retval = append(retval, '%')
			} else {
				if layoutCmd, ok := conversion[rune(c)]; ok {
					retval = append(retval, []byte(t.Format(layoutCmd))...)
				} else {
					retval = append(retval, '%', c)
				}
			}
		} else {
			if ni < len(format) {
				retval = append(retval, '%')
			}
		}
	}
	return string(retval)
}

var osLibrary = []RegistryFunction{
	{"clock", clock},
	{"date", func(l *State) int {
		// If no format string is provided then default to %c
		formatString := OptString(l, 1, "%c")
		intTime := OptInteger(l, 2, -1)

		var parsedTime time.Time
		if intTime == -1 {
			parsedTime = time.Now()
		} else {
			parsedTime = time.Unix(int64(intTime), 0)
		}

		// Process time in UTC
		if formatString[0] == '!' {
			// Delete the ! character
			formatString = formatString[1:len(formatString)]
			parsedTime = parsedTime.UTC()
		}

		year, month, day := parsedTime.Date()
		hour, min, sec := parsedTime.Clock()
		wday := int(parsedTime.Weekday())
		yday := parsedTime.YearDay()

		if formatString == "*t" {
			l.CreateTable(8, 0)

			l.PushInteger(sec)
			l.SetField(-2, "sec")

			l.PushInteger(min)
			l.SetField(-2, "min")

			l.PushInteger(hour)
			l.SetField(-2, "hour")

			l.PushInteger(day)
			l.SetField(-2, "day")

			l.PushInteger(int(month))
			l.SetField(-2, "month")

			l.PushInteger(year)
			l.SetField(-2, "year")

			l.PushInteger(wday)
			l.SetField(-2, "wday")

			l.PushInteger(yday)
			l.SetField(-2, "yday")
		} else {
			l.PushString(Format(formatString, parsedTime))
		}
		return 1
	}},
	{"difftime", func(l *State) int {
		l.PushNumber(time.Unix(int64(CheckNumber(l, 1)), 0).Sub(time.Unix(int64(OptNumber(l, 2, 0)), 0)).Seconds())
		return 1
	}},
	{"execute", func(l *State) int {
		c := OptString(l, 1, "")
		if c == "" {
			// TODO check if sh is available
			l.PushBoolean(true)
			return 1
		}
		cmd := strings.Fields(c)
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			// TODO
		}
		l.PushBoolean(true)
		l.PushString("exit")
		l.PushInteger(0)
		return 3
	}},
	{"exit", func(l *State) int {
		var status int
		if l.IsBoolean(1) {
			if !l.ToBoolean(1) {
				status = 1
			}
		} else {
			status = OptInteger(l, 1, status)
		}
		// if l.ToBoolean(2) {
		// 	Close(l)
		// }
		os.Exit(status)
		panic("unreachable")
	}},
	{"getenv", func(l *State) int { l.PushString(os.Getenv(CheckString(l, 1))); return 1 }},
	{"remove", func(l *State) int { name := CheckString(l, 1); return FileResult(l, os.Remove(name), name) }},
	{"rename", func(l *State) int { return FileResult(l, os.Rename(CheckString(l, 1), CheckString(l, 2)), "") }},
	// {"setlocale", func(l *State) int {
	// 	op := CheckOption(l, 2, "all", []string{"all", "collate", "ctype", "monetary", "numeric", "time"})
	// 	l.PushString(setlocale([]int{LC_ALL, LC_COLLATE, LC_CTYPE, LC_MONETARY, LC_NUMERIC, LC_TIME}, OptString(l, 1, "")))
	// 	return 1
	// }},
	{"time", func(l *State) int {
		if l.IsNoneOrNil(1) {
			l.PushNumber(float64(time.Now().Unix()))
		} else {
			CheckType(l, 1, TypeTable)
			l.SetTop(1)
			year := field(l, "year", -1)
			month := field(l, "month", -1)
			day := field(l, "day", -1)
			hour := field(l, "hour", 12)
			min := field(l, "min", 0)
			sec := field(l, "sec", 0)
			// dst := boolField(l, "isdst") // TODO how to use dst?

			l.PushNumber(float64(time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local).Unix()))
		}
		return 1
	}},
	{"tmpname", func(l *State) int {
		f, err := ioutil.TempFile("", "lua_")
		if err != nil {
			Errorf(l, "unable to generate a unique filename")
		}
		defer f.Close()
		l.PushString(f.Name())
		return 1
	}},
}

// OSOpen opens the os library. Usually passed to Require.
func OSOpen(l *State) int {
	NewLibrary(l, osLibrary)
	return 1
}
