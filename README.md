# storage

## Intent

The intent of this library is to help us scale by automatically caching / cache invalidating db requests. A few rules I have placed on writing this library are that:
1. **Automatically deal with all caching and cache invalidation**
2. It must be simpler to use this library than something like sqlx. Yes, we're that ambitious.
2. It should have transaction support, including dealing with cache invalidations
3. Some fine-grain controls on how you want your data to be in Redis e.g. LRange, RPush, Get, Set, etc.

## How It Works

The basic principle is that a struct in golang is essentially a row on in a table e.g. 
```
type Lead struct {
    LeadID int32 `json:"lead_id"`
    UserID int32 `json:"user_id"`
    GroupID int32 `json:"group_id"`
    Name string `json:"name"`
}

l := &Lead{
    LeadID: 100,
    UserID: 2,
    GroupID: 15,
    Name: "Jim the 2nd"
}
```

corresponds to this specific row:

lead_id | user_id | group_id | name
--- | --- | --- | ---
99 | 2 | 15 | "Jim the first"
**100** | **2** | **15** | **"Jim the 2nd"**
101 | 2 | 15 | "Jim the 3rd"

So when you have a the Lead struct filled out, it's a row in the Leads table.

Now, caching something such as `select * from leads where lead_id=100` is pretty straight forward. However, what happens if you want to do something like `select * from leads where group_id=15`. This becomes harder to cache for the obvious reasons:
1. If it's a list of objects, how do you get specific objects in it e.g. how do I get the first three?
2. Cache validation: if you're storing the full array then what happens if one gets updated?

### <ins>List vs. Struct</ins>

Foundationally in this library, all cached keys are either a list of primary keys or a struct. You never store slices of structs in Redis and instead create a list via LPush or RPush w/ the primary keys

I'm going to create two cache keys for the above queries:
1. `leads|lead_id:100` will represent the results of query `select * from leads where lead_id=100`
2. `leads|group_id:15` will represent the results of query `select * from leads where group_id=15`

In the Storage library, 
1. `leads|lead_id:100` would have the value of `{"lead_id": 100, "user_id": 2, "group_id": 15, Name: "Jim the 2nd"}`
2. `leads|group_id:15` would have a value of a list with values 99, 100, and 101 in them.

The reason for this is that each Query in the storage library defines what happens when a row is inserted, updated, or selected. For `leads|lead_id:%v` key, the `Query.InsertAction` will probably be a `CacheSet`. For `leads|group_id:%v` the `Query.InsertAction` is probably RPush because that'll ensure the order of the list is accurate. Therefore, assuming we insert another row such as 

```l := &Lead{
    UserID: 2,
    GroupID: 14,
    Name: Jim the 4th
}
storage.Insert(context.Background(), l)
```

then the returning object will be filled out with the corresponding lead_id (insert & update statements should always end in `RETURNING *`). So `l` will be:
```
l := &Lead{
    LeadID: 102
    UserID: 2,
    GroupID: 14,
    Name: Jim the 4th
}
```

Ok, now in the storage library it takes that row and cycles through all the available keys to that table and asks "do I have a matching key". If it does then it'll follow the `Query.InsertAction`. E.g.

`leads|lead_id:%v`. Well, yep, we have the lead_id field. `Query.InsertAction` is set so it'll take the key name `leads|lead_id:102` and set the value to be the json value of the field. Similarly, `leads|group_id:%v`. Yep, we've got a group_id in the struct. The action is RPush (add it to the rear of the list) so then `leads|group_id:14` gets added 102.

**note: RPush and LPush are actually RPushX and LPushX. If the key doesn't exist then the key will not be created. Only on select will the key actually be filled out for a list**

This is why when doing a storage.SelectAll, the options parameter has a FetchAll which will grab the primary keys from the list and either return them directly or actually fetch all of those keys from the cache and then return a populated slice of structs.

In this way, we can keep the cache up to date during updates & inserts

### <ins>Cached List Results</ins>

There's an issue that you might see pretty quickly: when we want to query a lot of something via SelectAll and FetchAll in the options is true then we're going through a ton of keys potentially. This could actually slow down the results relative to just doing a query. So why not just cache the whole result based off the limit & offset? And that's exactly what we do.

Every time SelectAll is called with FetchAll = true then we cache the full results as a struct so that the next time it's called it actually returns that full info vs. having to go through the list & fetch piece-meal all the data & returning.

Every list has two internal keys:
1. a key for using offset & limit (e.g. there'd be a `leads|groups:14|offset:%v|limit%v`)
2. a `metadata` key (e.g. there'd be a `leads|group_id:14|metadata` list)

The offsetLimit key is set with the full results if we're fetching all the data as a struct. It's then placed into the `metadata` list so that when there's an action that affects this key (e.g. an update or insert) then the cache can go to the `metadata` key and delete everything so that query is no longer cached. Boom.

### <ins>Updates</ins>

On the table, there's a field called `UpdateQuery`. This is supposed to be the query that's used when doing an update. The way this works is that first you get the full row(s) that you want by querying the storage library. Once you have each one, you then update the fields to what you want and then simply call the Update (or TXUpdate) function and it'll update that row.

The reason why this is the case vs. having a key are a few:
1. Security. You can't just randomly update all rows
2. Flexibility. It's just easy as hell to use it this way and if you do need to update multiple rows then you just range through each one & update.

## Implementation

Please see `examples/basic_service` first. It has a detailed readme thankfully (yep, I actually made documentation)

## TODO (in no particular order)
- Debugger needs to be re-written becuase it will interfere w/ other requests coming in. Since it's global, if multiple requests come in at the same time it'll cause issues
- Support only inserting certain fields into the cache
- Proto message support to reduce memory
- Support cache clusters (I have to look if this is already supported actually. This might already be enabled)
- REFACTOR SelectAll (note: there's a race condition when doing LPush & potential inserts too. This would be where someone selects all, it's not in cache, gets from DB, someone else does insert or someone else does a selectall, and then there's an invalidation. **Need to fix this badly**)
- Cache type of increment
- Allow for = validators to equal a sepicific value e.g. `role=OWNER`
- More validation checks during both runtime and during initialization. Off the top of my head:
    1. Make sure that no TTL is 0. Nothing should be cached permanently
- Unit tests / fuzzy testing would be nice...
- Lists should support sub-lists e.g. instead of a primary key stored, it should actually support a key that's a list in & of itself. This is **incredibly** useful for doing analytics where you can kinda do caching based off a lego system. Basically if you're trying to do something like an average number of chats per day over a month then you can think of that as an avg of avg's on a daily basis. Then the day is built of an avg of hours, which is an avg of minutes. Then the it builds up to the end-query (which eventually gets stored in its own right due to the metadata list). I **think** this is innately supported through the idea of referencedKeys too so if you have ListA which is comprised of ListB's and listB has an insert then with reference keys it should still update listA. I think. Gotta think that through if we run into it but timeseries and analytics aren't really high on our priority list (much like this whole library wasn't on our priority list but I made it, lmfao)
- Integration directly into sqlx / somehow move raw bytes directly from postgres to Redis automatically. That will save tons of Reflection for json transformations