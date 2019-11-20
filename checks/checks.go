package checks

import (
	"fmt"
	"regexp"
	"strconv"

	"database/sql"

	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
)

// InitDb initialize the database connection
func (c *Client) InitDb(db *rds.DBInstance, password, dbname string) {
	port := strconv.FormatInt(*db.Endpoint.Port, 10)
	host := *db.Endpoint.Address
	engine := *db.Engine
	user := *db.MasterUsername

	var err error
	args := fmt.Sprintf("host=%s port=%s user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	c.DB, err = sql.Open(engine, args)
	if err != nil {
		log.WithError(err).Error("Couldn't open connection to database")
		return
	}

	err = c.DB.Ping()
	if err != nil {
		log.WithError(err).Error("Couldn't ping database")
		return
	}
}

// CheckRegexAgainstRow will compare the regex and queries set in the yaml configuration file
// against each others
func (c *Client) CheckRegexAgainstRow(query, regex string) bool {

	rows, err := c.DB.Query(query)
	if err != nil {
		log.WithError(err).Error("Could not return db rows")
		return false
	}
	defer rows.Close()

	cols, _ := rows.Columns()

	data := make(map[string]string)

	for rows.Next() {
		columns := make([]string, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}
		err := rows.Scan(columnPointers...)

		for i, colName := range cols {
			data[colName] = columns[i]
		}

		if err == sql.ErrNoRows {
			log.WithError(err).Error("No Results Found")
			return false
		}

		if err != nil {
			log.WithError(err).Error("Could not scan db rows")
			return false
		}

		for _, result := range data {
			value, err := regexp.MatchString(regex, result)
			if err != nil {
				log.WithError(err).Error("Could not check regex against result")
				return false
			}
			for value {
				log.WithFields(log.Fields{
					"regex":  regex,
					"result": result,
				}).Info("Found a match")
				break
			}
			return value
		}
	}
	return true
}
