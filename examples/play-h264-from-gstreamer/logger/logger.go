package logger

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// Default log format will output [INFO]: 2006-01-02T15:04:05Z07:00 - Log message
	defaultLogPrefix       = "%time%, %lvl%, FCID=TBD"
	defaultLogFormat       = "%time%, %lvl%, FCID=TBD, %msg%"
	defaultTimestampFormat = time.RFC3339
)

// Formatter implements logrus.Formatter interface.
type Formatter struct {
	// Timestamp format
	TimestampFormat string
	// Available standard keys: time, msg, lvl
	// Also can include custom fields but limited to strings.
	// All of fields need to be wrapped inside %% i.e %time% %msg%
	LogFormat string
}

// Format building log message.
func (f *Formatter) Format(entry *logrus.Entry) ([]byte, error) {
	output := f.LogFormat
	if output == "" {
		output = defaultLogPrefix
	}

	timestampFormat := f.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = defaultTimestampFormat
	}

	output = strings.Replace(output, "%time%", entry.Time.Format(timestampFormat), 1)

	output = strings.Replace(output, "%msg%", entry.Message, 1)

	level := strings.ToUpper(entry.Level.String())
	output = strings.Replace(output, "%lvl%", level, 1)

	keys := make([]string, 0, len(entry.Data))
	for k := range entry.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		switch v := entry.Data[k].(type) {
		case string:
			output += fmt.Sprintf(", %s=%s", k, v)
		case int:
			s := strconv.Itoa(v)
			output += fmt.Sprintf(", %s=%s", k, s)
		case bool:
			s := strconv.FormatBool(v)
			output += fmt.Sprintf(", %s=%s", k, s)
		}
	}
	output += fmt.Sprintf(", msg=%s\n", entry.Message)

	return []byte(output), nil
}
