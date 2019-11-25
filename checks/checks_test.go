package checks

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestCheckRegexAgainstRow_stringTrue(t *testing.T) {
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

	value := c.CheckRegexAgainstRow("SELECT tablename FROM pg_catalog.pg_tables", "^pg_statistic$")
	assert.True(t, value)
}

func TestCheckRegexAgainstRow_stringFalse(t *testing.T) {
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

	value := c.CheckRegexAgainstRow("SELECT tablename FROM pg_catalog.pg_tables", "^fails$")
	assert.False(t, value)
}

func TestCheckRegexAgainstRow_int64True(t *testing.T) {
	db, mockdb, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	c := &Client{
		DB: db,
	}

	integer1 := int64(42)
	integer2 := int64(666)

	rows := sqlmock.NewRows([]string{"number"}).
		AddRow(integer1).
		AddRow(integer2)

	mockdb.ExpectQuery("SELECT number FROM database").WillReturnRows(rows)

	value := c.CheckRegexAgainstRow("SELECT number FROM database", "^42$")
	assert.True(t, value)
}

func TestCheckRegexAgainstRow_int64False(t *testing.T) {
	db, mockdb, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	c := &Client{
		DB: db,
	}

	integer1 := int64(42)
	integer2 := int64(666)

	rows := sqlmock.NewRows([]string{"number"}).
		AddRow(integer1).
		AddRow(integer2)

	mockdb.ExpectQuery("SELECT number FROM database").WillReturnRows(rows)

	value := c.CheckRegexAgainstRow("SELECT number FROM database", "^99$")
	assert.False(t, value)
}
