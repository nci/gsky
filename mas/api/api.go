// Metadata API
// Copyright (c) 2017, NCI, Australian National University.

package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq"
	"github.com/nci/gomemcache/memcache"
)

var (
	db       *sql.DB
	mc       *memcache.Client
	dbName   = flag.String("database", "mas", "database name")
	dbUser   = flag.String("user", "api", "database user name")
	dbPool   = flag.Int("pool", 8, "database pool size")
	dbLimit  = flag.Int("limit", 64, "database concurrent requests")
	httpPort = flag.Int("port", 8080, "http port")
	mcURI    = flag.String("memcache", "", "memcache uri host:port")
)

// Spit out a simple JSON-formatted error message for Content-Type: application/json
func httpJSONError(response http.ResponseWriter, err error, status int) {
	http.Error(response, fmt.Sprintf(`{ "error": %q }`, err.Error()), status)
}

func handler(response http.ResponseWriter, request *http.Request) {

	response.Header().Set("Content-Type", "application/json")

	var hash string

	if mc != nil {

		buff := md5.Sum([]byte(request.URL.RequestURI()))
		hash = hex.EncodeToString(buff[:])

		if cached, ok := mc.Get(hash); ok == nil {
			response.Write(cached.Value)
			return
		}
	}

	query := request.URL.Query()

	if _, ok := query["intersects"]; ok {

		var payload string

		// Use Postgres prepared statements and placeholders for input checks.
		// The nullif() noise is to coerce Go's empty string zero values for
		// missing parameters into proper null arguments.
		// The string_to_array() call will return null in the case of a null
		// argument, rather than array[] or array[null].

		err := db.QueryRow(
			`select mas_intersects(
				nullif($1,'')::text,
				nullif($2,'')::text,
				nullif($3,'')::text,
				nullif($4,'')::integer,
				nullif($5,'')::timestamptz,
				nullif($6,'')::timestamptz,
				string_to_array(nullif($7,''), ','),
				nullif($8,'')::numeric,
				nullif($9,'')::text
			) as json`,
			request.URL.Path,
			request.FormValue("srs"),
			request.FormValue("wkt"),
			request.FormValue("nseg"),
			request.FormValue("time"),
			request.FormValue("until"),
			request.FormValue("namespace"),
			request.FormValue("resolution"),
			request.FormValue("metadata"),
		).Scan(&payload)

		if err != nil {
			httpJSONError(response, err, 400)
			return
		}

		response.Write([]byte(payload))

		if mc != nil {
			// don't care about errors; memcache may not necessarily retain this anyway
			mc.Set(&memcache.Item{Key: hash, Value: []byte(payload)})
		}

		return
	}

	httpJSONError(response, errors.New("unknown operation; currently supported: ?intersects"), 400)
}

func main() {

	flag.Parse()

	log.Printf("dbUser %s dbName %s dbPool %d httpPort %d", *dbUser, *dbName, *dbPool, *httpPort)

	dbinfo := fmt.Sprintf("user=%s host=/var/run/postgresql dbname=%s sslmode=disable", *dbUser, *dbName)

	var err error
	db, err = sql.Open("postgres", dbinfo)

	if err != nil {
		panic(err)
	}

	defer db.Close()

	db.SetMaxIdleConns(*dbPool)
	db.SetMaxOpenConns(*dbLimit)

	if *mcURI != "" {
		// lazy connection; errors returned in .Get
		mc = memcache.New(*mcURI)
	}

	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *httpPort), nil))
}
