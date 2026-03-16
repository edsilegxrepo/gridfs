// Package main is the entry point for the GridFS Data Extractor.
// It handles command-line arguments, initializes configuration, and
// manages concurrent download workers to extract files from MongoDB GridFS.
package main

import (
	"context"
	"criticalsys/gridfs/pkg/config"
	"criticalsys/gridfs/pkg/fileops"
	"criticalsys/gridfs/pkg/gridfs"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var version string

func main() {
	// Define command-line flags
	configFile := flag.String("config", "", "Path to property file")
	blobList := flag.String("bloblist", "", "List of blob files to be retrieved")
	blobPath := flag.String("blobpath", "", "Path where to store the blob files")
	showVersion := flag.Bool("version", false, "Display application version")

	flag.Parse()

	// Argument validation
	if *showVersion {
		fmt.Println("GridFS Data Extractor:", version)
		return
	}
	if *configFile == "" || *blobList == "" || *blobPath == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -config <config_file> -bloblist <list_of_blob_files> -blobpath <Stored_blob_path>\n", filepath.Clean(os.Args[0]))
		os.Exit(1)
	}

	// Read configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to read configuration: %v", err)
	}

	// Read file names from the provided file
	fileNames, err := fileops.ReadFileNames(*blobList)
	if err != nil {
		log.Fatalf("Failed to read file names: %v", err)
	}

	// Prepare environment for download
	// Check destination blob path
	if err := fileops.CreateDirectory(*blobPath); err != nil {
		log.Fatalf("Error creating directory: %v", err)
	}

	// Initialize MongoDB connection
	// Create a new client and connect to the server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := gridfs.NewClient(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create GridFS client: %v", err)
	}
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			log.Fatalf("Failed to disconnect from MongoDB: %v", err)
		}
	}()

	// Orchestrate concurrent file downloads
	// Concurrently download files
	var wg sync.WaitGroup
	jobs := make(chan string, len(fileNames))

	// Launch worker pool
	for i := 0; i < cfg.NumWorkers; i++ {
		wg.Add(1)
		go worker(&wg, jobs, client, *blobPath, cfg)
	}

	// Distribute download tasks
	for _, fileName := range fileNames {
		jobs <- fileName
	}
	close(jobs)

	wg.Wait()

	fmt.Println("All files downloaded.")
}

// worker processes download tasks from the jobs channel.
// It checks for file existence before attempting a download to avoid redundant work.
func worker(wg *sync.WaitGroup, jobs <-chan string, client *gridfs.Client, blobPath string, cfg *config.Config) {
	defer wg.Done()
	for fileName := range jobs {
		// Construct the full path to the file
		filePath := filepath.Join(blobPath, fileName)

		if fileops.FileExistsAndNotEmpty(filePath) {
			log.Printf("File %s already exists locally and is not empty.\n", fileName)
			continue
		}

		err := client.DownloadFile(context.Background(), fileName, blobPath, int64(cfg.LargeFileThresholdMB)*1024*1024)
		if err != nil {
			log.Printf("Failed to download file %v: %v", fileName, err)
		} else {
			fmt.Printf("Downloaded file from gridfs to local: %v\n", fileName)
		}
	}
}
