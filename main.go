package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"pelagica-studios/internal/enrich"
	"pelagica-studios/internal/productioncompanies"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	idsFilePath, err := productioncompanies.DownloadProductionCompanyIds()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Saved production company export to %s\n", idsFilePath)

	processed, err := enrich.ProcessProductionCompanies(ctx, idsFilePath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Processed %d production companies\n", processed)
}
