package store

import storage "github.com/osr-alliance/backend-lib-storage"

var leadsGetByID = &storage.Query{
	Name:     LeadsGetByID,
	CacheKey: "service:basic_service|leads|lead_id:%v",

	Query: "select * from leads where lead_id=:lead_id",

	CacheTTL: DefaultTTL,

	InsertAction: storage.CacheSet,
	UpdateAction: storage.CacheSet,
	SelectAction: storage.CacheSet,
}

var leadsGetByUserID = &storage.Query{
	Name:                    LeadsGetByUserID,
	CacheKey:                "service:basic_service|leads|user_id:%v",
	CachePrimaryQueryStored: LeadsGetByID,

	Query: "select * from leads where user_id=:user_id",

	CacheTTL: DefaultTTL,

	InsertAction: storage.CacheRPush,
	UpdateAction: storage.CacheNoAction, // do not update the cache becuase when a lead is updated, the user's list of leads does not change
	SelectAction: storage.CacheRPush,
}

const leadsInsert = `INSERT INTO leads (user_id, name, email, phone, notes) 
VALUES 
(:user_id, :name, :email, :phone, :notes) RETURNING *` // note: make sure it's RETURNING *

const leadsUpdate = `update leads set notes=:notes where lead_id=:lead_id RETURNING *` // note: make sure it's RETURNING *
