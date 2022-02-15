package store

import storage "github.com/osr-alliance/backend-lib-storage"

// define all the query names we will use
const (
	/*
		It's standard to have the query used to fetch by
		the primary key be called {tableName}GetByID

		In this case, it's LeadsGetByID
	*/
	LeadsGetByID        = "LeadsGetByID"
	LeadsGetByUserID    = "LeadsGetByUserID"
	LeadsUpdateNameByID = "LeadsUpdateNameByID"
)

const (
	DefaultTTL = (3600 * 24 * 7) // 7 days
)

var leadsTable = &storage.Table{
	Struct:           Leads{},
	PrimaryQueryName: LeadsGetByID,
	PrimaryKeyField:  "lead_id",
	InsertQuery:      leadsInsert,
	UpdateQuery:      leadsUpdate,
	Queries: []*storage.Query{
		leadsGetByID,
		leadsGetByUserID,
	},
	ReferencedQueries: []*storage.Query{},
}
