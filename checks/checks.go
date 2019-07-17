package checks

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"

	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
	"github.com/techdroplabs/rdscheck/database"
)

// CheckSQLQueries takes a regex, a sql query and compare the result of the query
// against the regex
func CheckSQLQueries(i *rds.RDS, snap *rds.DBSnapshot, host rds.Endpoint, user, password, dbname, query, regex string) bool {

	p := strconv.FormatInt(*host.Port, 10)
	e := fmt.Sprintf("%s", *host.Address)

	database.InitDb(e, p, user, password, dbname)
	defer database.DB.Close()

	q := query
	var result string
	err := database.DB.QueryRow(q).Scan(&result)

	if err == sql.ErrNoRows {
		log.WithError(err).Error("No Results Found")
		return false
	}

	if err != nil {
		log.Error(err)
		return false
	}

	matched, err := regexp.MatchString(regex, result)
	if err != nil {
		log.WithError(err).Error("Could not check regex against result")
		return false
	}
	return matched
}
