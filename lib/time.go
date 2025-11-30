package lib

import (
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	Day  = 24 * time.Hour
	Year = 31622400 * time.Second
)

func formatDateTime(ts pgtype.Timestamptz) string {
	return ts.Time.In(time.UTC).Format(time.RFC3339)
}

func formatID[I ~int64 | ~int32](id I) string {
	return strconv.FormatInt(int64(id), 10)
}

func parseID[I ~int64 | ~int32](id string) I {
	if id == "" {
		return 0
	}
	raw, _ := strconv.ParseInt(id, 10, 32)
	return I(raw)
}
