package models

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// Date is a nullable date that serializes as "YYYY-MM-DD" in JSON.
type Date struct {
	Time  time.Time
	Valid bool
}

func NewDate(t time.Time) Date {
	return Date{Time: t, Valid: true}
}

func (d Date) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte("null"), nil
	}
	return []byte(`"` + d.Time.Format("2006-01-02") + `"`), nil
}

func (d *Date) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "null" {
		d.Valid = false
		return nil
	}
	t, err := time.Parse(`"2006-01-02"`, s)
	if err != nil {
		return fmt.Errorf("invalid date format, expected YYYY-MM-DD")
	}
	d.Time = t
	d.Valid = true
	return nil
}

func (d Date) Value() (driver.Value, error) {
	if !d.Valid {
		return nil, nil
	}
	return d.Time, nil
}

func (d *Date) Scan(value any) error {
	if value == nil {
		d.Valid = false
		return nil
	}
	t, ok := value.(time.Time)
	if !ok {
		return fmt.Errorf("cannot scan %T into Date", value)
	}
	d.Time = t
	d.Valid = true
	return nil
}
