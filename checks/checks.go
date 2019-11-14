package checks

import (
	"fmt"
	"regexp"
	"strconv"

	"database/sql"

	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
)

func (c *Client) InitDb(endpoint rds.Endpoint, user, password, dbname string) {

	port := strconv.FormatInt(*endpoint.Port, 10)
	host := *endpoint.Address

	var err error
	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	c.DB, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		log.WithError(err).Error("Couldn't open connection to postgres database")
		return
	}

	err = c.DB.Ping()
	if err != nil {
		log.WithError(err).Error("Couldn't ping postgres database")
		return
	}

	defer c.DB.Close()
}

// CheckSQLQueries takes a regex, a sql query and compare the result of the query
// against the regex
func (c *Client) CheckSQLQueries(query, regex string) bool {

	var result string
	rows, err := c.DB.Query(query)
	if err != nil {
		log.WithError(err).Error("Could not return db rows")
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&result)

		if err == sql.ErrNoRows {
			log.WithError(err).Error("No Results Found")
			return false
		}

		if err != nil {
			log.WithError(err).Error("Could not scan db rows")
			return false
		}
	}

	matched, err := regexp.MatchString(regex, result)
	if err != nil {
		log.WithError(err).Error("Could not check regex against result")
		return false
	}
	return matched
}
