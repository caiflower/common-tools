package basic

import (
	"database/sql/driver"
	"encoding/xml"
	"fmt"
	"time"

	"github.com/caiflower/common-tools/pkg/logger"
)

type TimeStandard time.Time

func NewTimeStandard(str string) TimeStandard {
	t := TimeStandard{}
	if str != "" && str != "null" {
		t.UnmarshalJSON([]byte(`"` + str + `"`))
	}
	return t
}

func (t TimeStandard) String() string {
	if t.IsZero() {
		return ""
	}
	return t.Time().Format(TimeFormat)
}

func (t *TimeStandard) UTCString() string {
	if t.IsZero() {
		return ""
	}
	return t.Time().UTC().Format("2006-01-02T15:04:05Z")
}

func (t *TimeStandard) Time() time.Time {
	return time.Time(*t)
}

func (t *TimeStandard) UnmarshalJSON(data []byte) (err error) {
	now, timeErr := unmarshalJSONToJson(data)
	if timeErr != nil {
		return timeErr
	}
	if !now.IsZero() {
		now = now.Local()
		*t = TimeStandard(now)
	}
	return
}

func (t *TimeStandard) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("\"\""), nil
	}
	b := make([]byte, 0, len(TimeFormat)+2)
	b = append(b, '"')
	b = t.Time().AppendFormat(b, TimeFormat)
	b = append(b, '"')
	return b, nil
}

func (t *TimeStandard) IsZero() bool {
	return t.Time().IsZero()
}

func (t *TimeStandard) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	err := e.EncodeElement(t.String(), start)
	if err != nil {
		return err
	}
	return nil
}

func (t *TimeStandard) Scan(val interface{}) (err error) {
	if val == nil {
		return
	}
	if _time, ok := val.(time.Time); ok {
		*t = TimeStandard(_time)
	} else if _, ok := val.([]byte); ok {
		str := string(val.([]byte))
		if str == "" || str == "\"\"" || str == "null" || str == "0000-00-00 00:00:00" {
			return
		}
		now, err := time.ParseInLocation(TimeFormat, str, time.Local)
		if err != nil {
			logger.Error("time type convert error. %s", err)
		}
		*t = TimeStandard(now)
	} else {
		logger.Error("time type convert error. invalid value type")
		return fmt.Errorf("time type convert error. invalid value type")
	}
	return
}

func (t *TimeStandard) Value() (driver.Value, error) {
	if t.IsZero() {
		return `0000-00-00 00:00:00`, nil
	}
	return t.String(), nil
}
