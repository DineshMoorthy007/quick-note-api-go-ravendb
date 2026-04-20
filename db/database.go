package db

import (
	"log"
	"strings"

	ravendb "github.com/ravendb/ravendb-go-client"
)

func InitDB(serverURL string, databaseName string) *ravendb.DocumentStore {
	if strings.TrimSpace(serverURL) == "" {
		log.Printf("ravendb init failed: serverURL is empty")
		return nil
	}

	if strings.TrimSpace(databaseName) == "" {
		log.Printf("ravendb init failed: databaseName is empty")
		return nil
	}

	store := ravendb.NewDocumentStore([]string{serverURL}, databaseName)
	if err := store.Initialize(); err != nil {
		log.Printf("ravendb init failed for url=%s database=%s: %v", serverURL, databaseName, err)
		return nil
	}

	return store
}
