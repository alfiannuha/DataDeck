package handler

import (
	"errors"
	"testing"

	"github.com/datadeck/datadeck/backend/internal/model"
)

func profileFor(driver model.Driver, database string) *model.ConnectionProfile {
	return &model.ConnectionProfile{Driver: driver, DatabaseName: database}
}

func TestResolveTargetDatabase(t *testing.T) {
	pg := model.DriverPostgres
	my := model.DriverMySQL
	lite := model.DriverSQLite

	cases := []struct {
		name      string
		profile   *model.ConnectionProfile
		requested string
		want      string
		wantErr   error
	}{
		{"postgres request wins", profileFor(pg, "CCM"), "reporting", "reporting", nil},
		{"postgres falls back to default", profileFor(pg, "CCM"), "", "CCM", nil},
		{"postgres requires a database", profileFor(pg, ""), "", "", errDatabaseRequired},
		{"postgres request over empty default", profileFor(pg, ""), "reporting", "reporting", nil},
		{"mysql uses profile database", profileFor(my, "app"), "", "app", nil},
		{"mysql accepts matching database", profileFor(my, "app"), "app", "app", nil},
		{"mysql rejects mismatch", profileFor(my, "app"), "other", "", errDatabaseMismatch},
		{"sqlite uses file path", profileFor(lite, "/tmp/x.db"), "", "/tmp/x.db", nil},
		{"sqlite rejects mismatch", profileFor(lite, "/tmp/x.db"), "/tmp/y.db", "", errDatabaseMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveTargetDatabase(tc.profile, tc.requested)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("database = %q, want %q", got, tc.want)
			}
		})
	}
}
