# storage

## Intent

The intent of this library is to help us scale by automatically caching / cache invalidating db requests. A few rules I have placed on writing this library are that:
1. **Automatically deal with all caching and cache invalidation**
2. It must be simpler to use this library than something like sqlx. Yes, we're that ambitious.
2. It should have transaction support, including dealing with cache invalidations
3. Some fine-grain controls on how you want your data to be in Redis e.g. LRange, RPush, Get, Set, etc.

## Implementation

The first and foremost thing to know is that this is only supported with sqlx and Redis for the time being.

This is the part of the README that I really should type out but... ugh. Just fucking go see the opportunity service for now please :)

## TODO
- Support cache clusters (I have to look if this is already supported actually)
- Enable go routines (specifically when doing an s.action()) for improvements; I disabled for debugging purposes
- **needed feature**: a CacheDataStructureList should be able to store a primary key to another table as its range and get that table's info instead of its own 
- More validation checks during both runtime and during initialization e.g. checking to make sure SelectAction on the key matches its CacheDataStructure
- We might not actually need CacheDataStructure. Might be good to have though
- Enforce a higher degree of structure and validation on key e.g. we could have key for `service:opportunity|opportunities|OpportunityID:%v` automatically detect it's expecting the OpportunityID and fill it out during new() and fill in the Fields portion

Later versions, we could:
- Move raw bytes directly from postgres to Redis automatically. That will save tons of Reflection