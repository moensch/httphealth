package httphealth

import (
	"errors"
	"fmt"
	"time"
)

type Cache struct {
	Entries map[string]CacheEntry
}

type CacheEntry struct {
	Name     string
	Response CheckResponse
	Ttl      int64
	Created  int64
}

func (e *CacheEntry) Expires() int64 {
	return e.Created + e.Ttl
}

func (e *CacheEntry) ValidFor() int64 {
	return e.Expires() - time.Now().Unix()
}

func (c *Cache) Set(name string, resp CheckResponse, ttl int64) {
	fmt.Printf("CACHE: Caching %s for %d seconds\n", name, ttl)
	entry := CacheEntry{
		Name:     name,
		Response: resp,
		Ttl:      ttl,
		Created:  time.Now().Unix(),
	}

	if c.Entries == nil {
		c.Entries = make(map[string]CacheEntry)
	}

	c.Entries[name] = entry
}

func (c *Cache) Delete(name string) {
	// nil-safe
	fmt.Printf("CACHE: Removing entry %s\n", name)
	delete(c.Entries, name)
}

func (c *Cache) Get(name string) (CheckResponse, int64, error) {
	var err error
	var validFor int64
	resp := CheckResponse{}

	if entry, ok := c.Entries[name]; ok {
		// Found an entry, check ttl
		if entry.ValidFor() > 0 {
			// Still valid
			fmt.Printf("CACHE: Cache hit for %s, expires in %d\n", name, entry.ValidFor())
			return entry.Response, entry.ValidFor(), err
		} else {
			// Expired
			fmt.Printf("CACHE: Cache for %s expired %d ago\n", name, entry.ValidFor())
			c.Delete(entry.Name)
		}
	} else {
		fmt.Printf("CACHE: No entry for %s\n", name)
	}

	return resp, validFor, errors.New("Not found")
}
