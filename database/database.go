package database

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
)

var DB *sqlx.DB

func InitDb(host, port, user, password, dbname string) {
	var err error
	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	DB, err = sqlx.Open("postgres", psqlInfo)
	if err != nil {
		log.WithError(err).Error("Couldn't open connection to postgres database")
		return
	}

	err = DB.Ping()
	if err != nil {
		log.WithError(err).Error("Couldn't ping postgres database")
		return
	}
}
