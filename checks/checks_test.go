package checks

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/mock"
)

type mockDB struct {
	sqlmock.Sqlmock
	*sql.DB
	mock.Mock
}

func (m *mockDB) MatchString(pattern string, s string) (matched bool, err error) {
	args := m.Called(pattern, s)
	return args.Bool(0), args.Error(1)
}

func TestInitDb(t *testing.T) {

}

func TestCheckSQLQueries(t *testing.T) {
	db, mockdb, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	database := &mockDB{}

	c := &Client{
		DB: db,
	}

	rows := sqlmock.NewRows([]string{"tablename"}).
		AddRow("pg_statistic").
		AddRow("pg_type")

	mockdb.ExpectQuery("SELECT tablename FROM pg_catalog.pg_tables").WillReturnRows(rows)

	database.On("MatchString", mock.Anything, mock.Anything).Return(true, nil)

	c.CheckSQLQueries("SELECT tablename FROM pg_catalog.pg_tables", "^pg_statistic$")
}
