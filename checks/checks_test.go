package checks

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestCheckRegexAgainstRow(t *testing.T) {
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

	// rows, err = c.DB.Query("SELECT tablename FROM pg_catalog.pg_tables")
	// assert.Nil(t, err)

	value := c.CheckRegexAgainstRow("SELECT tablename FROM pg_catalog.pg_tables", "^pg_statistic$")
	assert.True(t, value)
}
