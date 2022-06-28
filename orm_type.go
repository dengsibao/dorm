package dorm

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"github.com/sirupsen/logrus"
	"regexp"
	"strings"
	"time"
)

const (
	DateLayout = "2006-01-02"
	TimeLayout = "15:04:05"
	DatetimeLayout = DateLayout + " " + TimeLayout
)

type Time struct {
	sql.NullTime
}

func (t *Time) MarshalJSON() ([]byte, error) {
	if t.Valid {
		return json.Marshal(t.Time.Format(DatetimeLayout))
	}
	return json.Marshal(nil)
}

func (t *Time) String() string {
	if t.Valid {
	 return t.Time.Format(DatetimeLayout)
	}
	return ""
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// The time is expected to be a quoted string in RFC 3339 format.
func (t *Time) UnmarshalJSON(data []byte) (err error) {
	if bytes.Equal(data, []byte("null")) {
		return nil
	} else {
		var v = strings.TrimSpace(strings.ReplaceAll(string(data), "\"", ""))
		var layout = ""
		if ok, _ := regexp.Match(`^\d{4}-\d{1,2}-\d{1,2}$`, []byte(v)); ok {
			layout = DateLayout
		}
		if ok, _ := regexp.Match(`^\d{4}-\d{1,2}-\d{1,2} \d{2}:\d{2}:\d{2}$`, []byte(v)); ok {
			layout = DatetimeLayout
		}
		if layout == "" {
			return nil
		}
		tt, err := time.ParseInLocation(layout, v, time.Local)
		if err != nil {
			logrus.Errorf("run here parse time error: %v", err)
			return err
		}
		*t = Time{sql.NullTime{Valid: true, Time: tt}}
		return nil
	}
}

func (t *Time) GetDate() string {
	if t.Valid {
		return t.Time.Format(DateLayout)
	}
	return ""
}

func (t *Time) GetDateTime() string {
	if t.Valid {
		return t.Time.Format(DatetimeLayout)
	}
	return ""
}

// Strings support []string
type Strings []string

func (c Strings) Value() (driver.Value, error) {
	b, err := json.Marshal(c)
	return string(b), err
}

func (c *Strings) Scan(input interface{}) error {
	if input == nil {
		return json.Unmarshal([]byte("[]"), c)
	}
	return json.Unmarshal(input.([]byte), c)
}

// Int64s support []int64
type Int64s []int64

func (c Int64s) Value() (driver.Value, error) {
	b, err := json.Marshal(c)
	return string(b), err
}

func (c *Int64s) Scan(input interface{}) error {
	if input == nil {
		return json.Unmarshal([]byte("[]"), c)
	}
	return json.Unmarshal(input.([]byte), c)
}