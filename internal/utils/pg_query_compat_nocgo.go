//go:build !cgo

package utils

import (
	"errors"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

var errPGQueryUnavailable = errors.New("postgres SQL parser unavailable: build with CGO_ENABLED=1")

// The upstream parser is CGO-backed. Keep the application buildable on
// platforms without CGO and fail closed at the validation boundary.
func parsePGSQL(string) (*pg_query.ParseResult, error)   { return nil, errPGQueryUnavailable }
func deparsePGSQL(*pg_query.ParseResult) (string, error) { return "", errPGQueryUnavailable }
