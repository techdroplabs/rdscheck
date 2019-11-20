package checks

import (
	"testing"
	"time"

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

func TestCheckRegexAgainstRow_timeTrue(t *testing.T) {
	db, mockdb, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	c := &Client{
		DB: db,
	}

	time1 := time.Now()
	string1 := time1.Format("2006-01-02T15:04:05.999999-07:00")
	regex1 := "^" + string1 + "$"
	time2 := time.Now().Add(time.Second * 3600)

	rows := sqlmock.NewRows([]string{"time"}).
		AddRow(time1).
		AddRow(time2)

	mockdb.ExpectQuery("SELECT time FROM database").WillReturnRows(rows)

	value := c.CheckRegexAgainstRow("SELECT time FROM database", regex1)
	assert.True(t, value)
}

func TestCheckRegexAgainstRow_timeFalse(t *testing.T) {
	db, mockdb, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	c := &Client{
		DB: db,
	}

	time1 := time.Now()
	time2 := time.Now().Add(time.Second * 3600)

	rows := sqlmock.NewRows([]string{"time"}).
		AddRow(time1).
		AddRow(time2)

	mockdb.ExpectQuery("SELECT time FROM database").WillReturnRows(rows)

	value := c.CheckRegexAgainstRow("SELECT time FROM database", "^2006-01-02T15:04:05.999999-07:00$")
	assert.False(t, value)
}
