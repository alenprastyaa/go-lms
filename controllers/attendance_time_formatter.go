package controllers

import "time"

const (
	jakartaDateLayout     = "2006-01-02"
	jakartaDateTimeLayout = "2006-01-02 15:04:05"
)

func normalizeAttendanceMap(row map[string]interface{}) {
	if len(row) == 0 {
		return
	}

	if value, ok := row["attendance_date"]; ok {
		row["attendance_date"] = normalizeJakartaDateValue(value)
	}
	if value, ok := row["clock_in"]; ok {
		row["clock_in"] = normalizeJakartaDateTimeValue(value)
	}
	if value, ok := row["clock_out"]; ok {
		row["clock_out"] = normalizeJakartaDateTimeValue(value)
	}
}

func normalizeAttendanceMaps(rows []map[string]interface{}) {
	for _, row := range rows {
		normalizeAttendanceMap(row)
	}
}

func normalizeJakartaDateTimeFields(row map[string]interface{}, fields ...string) {
	if len(row) == 0 {
		return
	}

	for _, field := range fields {
		if value, ok := row[field]; ok {
			row[field] = normalizeJakartaDateTimeValue(value)
		}
	}
}

func normalizeJakartaDateTimeRows(rows []map[string]interface{}, fields ...string) {
	for _, row := range rows {
		normalizeJakartaDateTimeFields(row, fields...)
	}
}

func normalizeJakartaDateValue(value interface{}) interface{} {
	switch t := value.(type) {
	case time.Time:
		return reinterpretAsJakartaClock(t).Format(jakartaDateLayout)
	case *time.Time:
		if t == nil {
			return nil
		}
		return reinterpretAsJakartaClock(*t).Format(jakartaDateLayout)
	case string:
		parsed := parseJakartaTimestamp(t)
		if parsed == nil {
			return t
		}
		return parsed.Format(jakartaDateLayout)
	default:
		return value
	}
}

func normalizeJakartaDateTimeValue(value interface{}) interface{} {
	switch t := value.(type) {
	case time.Time:
		return reinterpretAsJakartaClock(t).Format(jakartaDateTimeLayout)
	case *time.Time:
		if t == nil {
			return nil
		}
		return reinterpretAsJakartaClock(*t).Format(jakartaDateTimeLayout)
	case string:
		parsed := parseJakartaTimestamp(t)
		if parsed == nil {
			return t
		}
		return parsed.Format(jakartaDateTimeLayout)
	default:
		return value
	}
}

func reinterpretAsJakartaClock(value time.Time) time.Time {
	location := jakartaLocation()
	return time.Date(
		value.Year(),
		value.Month(),
		value.Day(),
		value.Hour(),
		value.Minute(),
		value.Second(),
		value.Nanosecond(),
		location,
	)
}
