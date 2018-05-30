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
	"github.com/bradfitz/gomemcache/memcache"
	_ "github.com/lib/pq"
	"log"
	"net/http"
)

var (
	db        *sql.DB
	mc        *memcache.Client
	db_name   = flag.String("database", "mas", "database name")
	db_user   = flag.String("user", "api", "database user name")
	db_pool   = flag.Int("pool", 8, "database pool size")
	db_limit  = flag.Int("limit", 64, "database concurrent requests")
	http_port = flag.Int("port", 8080, "http port")
	mc_uri    = flag.String("memcache", "", "memcache uri host:port")
)

// Spit out a simple JSON-formatted error message for Content-Type: application/json
func httpJsonError(response http.ResponseWriter, err error, status int) {
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
				nullif($4,'')::timestamptz,
				nullif($5,'')::timestamptz,
				string_to_array(nullif($6,''), ','),
				nullif($7,'')::numeric,
				nullif($8,'')::text
			) as json`,
			request.URL.Path,
			request.FormValue("srs"),
			request.FormValue("wkt"),
			request.FormValue("time"),
			request.FormValue("until"),
			request.FormValue("namespace"),
			request.FormValue("resolution"),
			request.FormValue("metadata"),
		).Scan(&payload)

		if err != nil {
			httpJsonError(response, err, 400)
			return
		}

		response.Write([]byte(payload))

		if mc != nil {
			// don't care about errors; memcache may not necessarily retain this anyway
			mc.Set(&memcache.Item{Key: hash, Value: []byte(payload)})
		}

		return
	}

	httpJsonError(response, errors.New("unknown operation; currently supported: ?intersects"), 400)
}

func main() {

	flag.Parse()

	log.Printf("db_user %s db_name %s db_pool %d http_port %d", *db_user, *db_name, *db_pool, *http_port)

	dbinfo := fmt.Sprintf("user=%s host=/var/run/postgresql dbname=%s sslmode=disable", *db_user, *db_name)

	var err error
	db, err = sql.Open("postgres", dbinfo)

	if err != nil {
		panic(err)
	}

	defer db.Close()

	db.SetMaxIdleConns(*db_pool)
	db.SetMaxOpenConns(*db_limit)

	if *mc_uri != "" {
		// lazy connection; errors returned in .Get
		mc = memcache.New(*mc_uri)
	}

	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *http_port), nil))
}
