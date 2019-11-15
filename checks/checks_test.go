package checks

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCheckSQLQueries(t *testing.T) {
	db, mockdb, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	c := &Client{
		DB: db,
	}

	rows := sqlmock.NewRows([]string{"tablename"}).
		AddRow("pg_statistic").
		AddRow("pg_type")

	mockdb.ExpectQuery("SELECT tablename FROM pg_catalog.pg_tables").WillReturnRows(rows)

	c.CheckSQLQueries("SELECT tablename FROM pg_catalog.pg_tables", "^pg_statistic$")
}
