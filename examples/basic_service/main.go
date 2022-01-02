package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/osr-alliance/backend-lib-storage/examples/basic_service/store"
	"github.com/sirupsen/logrus"
)

const (
	DBUSER = "postgres"
	DBPASS = "changeme"
	DBHOST = "localhost"
	DBPORT = "5432"
	DBNAME = "basic_service"

	REDISHOST = "localhost"
	REDISPASS = ""
	REDISPORT = "6379"
)

func main() {

	router := mux.NewRouter()

	writeConn, readConn, redisDb := createConns()

	l := NewLead(&Config{
		readConn:  readConn,
		writeConn: writeConn,
		redis:     redisDb,
	})

	router.HandleFunc("/leads/{id:[0-9]+}/get", l.Get)
	router.HandleFunc("/leads/set", l.Set)
	router.HandleFunc("/leads/update", l.Update)
	router.HandleFunc("/leads/{id:[0-9]+}/list", l.List)

	srv := &http.Server{
		Handler: router,
		Addr:    "127.0.0.1:8000",
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

func createConns() (*sqlx.DB, *sqlx.DB, *redis.Client) {
	// set logrus log level to debug
	logrus.SetLevel(logrus.DebugLevel)

	// Main DB setup
	connString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", DBHOST, DBPORT, DBUSER, DBPASS, DBNAME)

	// get our db connections
	db, err := sqlx.Connect("postgres", connString)
	if err != nil {
		panic(err)
	}
	if db == nil {
		panic("dbConn is nil")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", REDISHOST, REDISPORT),
		Password: REDISPASS, // no password set
		DB:       0,         // use default DB
	})
	if rdb == nil {
		log.Fatal("unable to connect to redis")
	}

	return db, db, rdb
}

type Config struct {
	writeConn *sqlx.DB
	readConn  *sqlx.DB
	redis     *redis.Client
}

type lead struct {
	store store.Store
}

type leadInterface interface {
	Get(w http.ResponseWriter, r *http.Request)
	Set(w http.ResponseWriter, r *http.Request)
	Update(w http.ResponseWriter, r *http.Request)
	List(w http.ResponseWriter, r *http.Request)
}

func NewLead(conf *Config) leadInterface {
	s := &store.Config{
		ReadConn:  conf.readConn,
		WriteConn: conf.writeConn,
		Redis:     conf.redis,
	}

	return &lead{
		store: store.New(s),
	}
}

func (l *lead) Get(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// convert id from string to int32
	idInt, err := strconv.ParseInt(id, 10, 32)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// now that we have the id, we can get the lead
	a, err := l.store.GetLeadByID(context.Background(), int32(idInt))
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	// should return json but w/e
	fmt.Fprintf(w, "%+v", a)
}

func (l *lead) Set(w http.ResponseWriter, r *http.Request) {
	lead := &store.Leads{}
	err := json.NewDecoder(r.Body).Decode(lead)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	/*
		NOTE: this is where your validation would go. I'm too lazy though...
	*/
	err = l.store.SetLead(context.Background(), lead)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	// should return json but w/e
	fmt.Fprintf(w, "%+v", lead)
}

func (l *lead) Update(w http.ResponseWriter, r *http.Request) {
	lead := &store.Leads{}
	err := json.NewDecoder(r.Body).Decode(lead)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	/*
		NOTE: this is where your validation would go. I'm too lazy though...
	*/
	lead, err = l.store.UpdateLeadsNotes(context.Background(), lead.LeadID, lead.Notes)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	// should return json but w/e
	fmt.Fprintf(w, "%+v", lead)
}

func (l *lead) List(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["id"]

	// convert id from string to int32
	idInt, err := strconv.ParseInt(userID, 10, 32)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// now that we have the id, we can get the lead
	a, err := l.store.GetLeadsByUserID(context.Background(), int32(idInt))
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	// should return json but w/e
	fmt.Fprintf(w, "%+v", a)
}
