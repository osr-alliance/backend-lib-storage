# Basic Service

This runs through a very basic implementation of the storage library

### DB

DB is a simple sql file. Nothing else in there except for a leads table that has some very basic info.

### Main.go

Very simply put: not a lot here. All we do is set up the redis & db conns, pass them to the "lead" service, and let it handle everything.

### leads/store

Alright, here's where things get interesting.

1. All services have a `store` package
2. `store.go`
    - Instantiates the store & storage package
    - Defines the interface for the store
        - Note: Do **NOT** expose the storage package to the service; it should all be wrapped behind an interface and store should do the actions to the storage
3. `store_X.go`
    - All implementations of the `store` interface that are related to X should be placed in `store_x.go`
        - e.g. all leads' queries and stuff should be placed in `store_leads.go`
4. `structs.go` defines all the structs for the store (you can think of a struct as a row in a table)
    - remember that the tag is `json` and not `db`
5. `tables.go`
    - Create names for all queries via an iota starting with `Default` at 0
    - Define all your *storage.Tables in there
6. `queries_X.go`
    - All queries related to X struct/table are defined here.
    - In examples/basic_service, there's a `queries_leads.go` file. This is because there's a `leads` struct so all queries related to leads are placed in here
    - Each query should be private version of the corresponding Query defined in the `tables.go`
        - e.g. if there's a `LeadsGetByID` query as an iota in `tables.go` then there should be a *storage.Query var named `leadsGetByID`
