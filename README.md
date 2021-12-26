# storage

## Intent

The intent of this library is to help us scale by automatically caching / cache invalidating db requests. A few rules I have placed on writing this library are that:
1. **Automatically deal with all caching and cache invalidation**
2. It must be simpler to use this library than something like sqlx. Yes, we're that ambitious.
2. It should have transaction support, including dealing with cache invalidations
3. Some fine-grain controls on how you want your data to be in Redis e.g. LRange, RPush, Get, Set, etc.

## Implementation

The first and foremost thing to know is that this is only supported with sqlx and Redis for the time being.

When using this library, it's really important to understand that most of this is going to be a configuration issue.

**IMPORTANT: a few notes to start**
1. Every table in your db should have one single corresponding struct
    * Note: the tags in sqlx for your struct is default to "db" but we opt to use "json" instead
2. A Table defines how the struct / table should interact with its keys / sql statements
3. Each of your Query defines a specific sql statement you want to make against one specific Table

For the purposes of this library, we're going to pretend that we have two tables:
1. `group`
2. `relation_group_user`

Group table will have a sql structure of:
```
GROUP
group_id serial PRIMARY KEY,
display_name VARCHAR(50),
group_type VARCHAR(50)
timestamp timestamp NOT NULL DEFAULT NOW(),
```

Relation_group_user table will have a sql structure of:
```
RELATION_GROUP_USER
relation_id serial PRIMARY KEY,
group_id int NOT NULL,
user_id int NOT NULL,
unique(group_id, user_id),
start_timestamp timestamp NOT NULL DEFAULT NOW(),
end_timestamp timestamp DEFAULT NULL,
is_admin boolean NOT NULL
```

When it comes to our Go code, we'll have **three** structs:
```
// Group is our public struct of our database table
type Group struct {
	GroupID        int32         `json:"group_id"`
	Name           string        `json:"display_name"`
	GroupType      string        `json:"group_type"`
	Timestamp      sql.NullTime  `json:"timestamp"`
}

type RelationGroupUser struct {
    relationID      int32           `json:"relation_id"`
    UserID          int32           `json:"user_id"`
    GroupID         int32           `json:"group_id"`
    IsAdmin         bool            `json:"is_admin"`
    StartTimestamp  sql.NullTime    `json:"start_timestamp"`
    EndTimestamp    sql.NullTime    `json:"end_timestamp"`
}

type UserIsAuthorized struct {
    UserID          int32   `json:"user_id"`
    GroupID         int32   `json:"group_id"`
    IsAuthorized    bool    `json:"is_authorized"`
}
```

### Table

You can think of a Table as a way to define & group all the queries that is ran off the the Table









## TODO
- Support cache clusters (I have to look if this is already supported actually)
- Enable go routines (specifically when doing an s.action()) for improvements; I disabled for debugging purposes
- **needed feature**: a CacheDataStructureList should be able to store a primary key to another table as its range and get that table's info instead of its own 
- More validation checks during both runtime and during initialization e.g. checking to make sure SelectAction on the key matches its CacheDataStructure

Later versions, we could:
- Move raw bytes directly from postgres to Redis automatically. That will save tons of Reflection