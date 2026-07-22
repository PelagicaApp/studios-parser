package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"pelagica-studios/internal/entity"
	"pelagica-studios/internal/enrich"
	"pelagica-studios/internal/exports"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	idsFiles := make(map[entity.Type]string)
	for _, entityType := range []entity.Type{entity.TypeProductionCompany, entity.TypeTVNetwork} {
		idsFilePath, err := exports.DownloadIDs(entityType)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Saved %s export to %s\n", entityType, idsFilePath)
		idsFiles[entityType] = idsFilePath
	}

	processed, err := enrich.Process(ctx, idsFiles)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Processed %d entities\n", processed)
}
