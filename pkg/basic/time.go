/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

 package basic

import (
	"database/sql/driver"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
)

const (
	TimeFormat  = "2006-01-02 15:04:05"
	TimeFormatT = "2006-01-02T15:04:05"
)

type Time time.Time

func NewTime(str string) Time {
	t := Time{}
	if str != "" && str != "null" {
		t.UnmarshalJSON([]byte(`"` + str + `"`))
	}
	return t
}

func init() {
	TimePattern1 = regexp.MustCompile("[.][0-9]+ [+][0-9]{4} CST\"$")
	TimePattern2 = regexp.MustCompile("[+][0-9]{2}:[0-9]{2}\"$")
	TimePattern3 = regexp.MustCompile("[.][0-9]+Z\"$")
	TimePattern4 = regexp.MustCompile("[:][0-9]{2}Z\"$")
	TimePattern5 = regexp.MustCompile("[.][0-9]+\"$")
}

var (
	TimePattern1 *regexp.Regexp
	TimePattern2 *regexp.Regexp
	TimePattern3 *regexp.Regexp
	TimePattern4 *regexp.Regexp
	TimePattern5 *regexp.Regexp
)

func unmarshalJSONToJson(data []byte) (time.Time, error) {
	str := string(data)
	if str == "\"\"" || str == "null" {
		return time.Time{}, nil
	}
	loc := time.UTC
	format := TimeFormat
	if strings.Index(str, "T") == 11 {
		format = TimeFormatT
	}
	byteStr := []byte(str)
	if TimePattern1.Match(byteStr) {
		format = format + ".999999999 Z0700 CST"
	} else if TimePattern2.Match(byteStr) {
		format = format + "Z07:00"
	} else if TimePattern3.Match(byteStr) {
		format = format + ".999999999Z"
	} else if TimePattern4.Match(byteStr) {
		format = format + "Z"
	} else if TimePattern5.Match(byteStr) {
		format = format + ".999999999"
		loc = time.Local
	} else {
		loc = time.Local
	}
	return time.ParseInLocation(`"`+format+`"`, str, loc)
}

func (t Time) String() string {
	if t.IsZero() {
		return ""
	}
	return t.Time().Format(TimeFormat)
}

func (t *Time) UTCString() string {
	if t.IsZero() {
		return ""
	}
	return t.Time().UTC().Format("2006-01-02T15:04:05Z")
}

func (t *Time) Time() time.Time {
	return time.Time(*t)
}

func (t *Time) UnmarshalJSON(data []byte) (err error) {
	now, timeErr := unmarshalJSONToJson(data)
	if timeErr != nil {
		logger.Error("time type convert error. %s", timeErr)
		return
	}
	if !now.IsZero() {
		now = now.Local()
		*t = Time(now)
	}
	return
}

func (t *Time) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("\"\""), nil
	}
	b := make([]byte, 0, len(TimeFormat)+2)
	b = append(b, '"')
	b = t.Time().AppendFormat(b, TimeFormat)
	b = append(b, '"')
	return b, nil
}

func (t *Time) IsZero() bool {
	return t.Time().IsZero()
}

func (t *Time) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	err := e.EncodeElement(t.String(), start)
	if err != nil {
		return err
	}
	return nil
}

func (t *Time) Scan(val interface{}) (err error) {
	if val == nil {
		return
	}
	if _time, ok := val.(time.Time); ok {
		*t = Time(_time)
	} else if _, ok := val.([]byte); ok {
		str := string(val.([]byte))
		if str == "" || str == "\"\"" || str == "null" || str == "0000-00-00 00:00:00" {
			return
		}
		now, err := time.ParseInLocation(TimeFormat, str, time.Local)
		if err != nil {
			logger.Error("time type convert error. %s", err)
		}
		*t = Time(now)
	} else {
		logger.Error("time type convert error. invalid value type")
		return fmt.Errorf("time type convert error. invalid value type")
	}
	return
}

func (t *Time) Value() (driver.Value, error) {
	if t.IsZero() {
		return `0000-00-00 00:00:00`, nil
	}
	return t.String(), nil
}
