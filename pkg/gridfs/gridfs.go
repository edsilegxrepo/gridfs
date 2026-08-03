// Package gridfs provides a simplified interface for interacting with MongoDB GridFS,
// supporting both memory-efficient streaming for large files and in-memory reading for small ones.
package gridfs

import (
	"context"
	"criticalsys.net/gridfs/pkg/config"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Client is a wrapper around the mongo client and bucket.
type Client struct {
	client *mongo.Client
	bucket *mongo.GridFSBucket
}

// NewClient creates a new GridFS client.
func NewClient(ctx context.Context, cfg *config.Config) (*Client, error) {
	clientOptions := options.Client().ApplyURI(cfg.MongoURI).SetReadPreference(readpref.Secondary()).SetAuth(options.Credential{
		Username: cfg.MongoUser,
		Password: cfg.MongoPass,
	})

	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := client.Database(cfg.MongoDB)
	bucket := db.GridFSBucket(options.GridFSBucket().SetName(cfg.MongoGridFSPrefix))

	return &Client{
		client: client,
		bucket: bucket,
	}, nil
}

// DownloadFile downloads a file from GridFS and saves it to the specified path.
// It implements a threshold-based strategy:
// - Files larger than largeFileThreshold are streamed directly to disk to save memory.
// - Smaller files are read into memory first for performance.
func (c *Client) DownloadFile(ctx context.Context, fileName, blobPath string, largeFileThreshold int64) (err error) {
	downloadStream, err := c.bucket.OpenDownloadStreamByName(ctx, fileName)
	if err != nil {
		return fmt.Errorf("failed to open download stream for file %v: %w", fileName, err)
	}
	defer func() {
		if closeErr := downloadStream.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	fileSize := downloadStream.GetFile().Length
	filePath := filepath.Join(blobPath, fileName)

	if fileSize > largeFileThreshold {
		// Stream large files
		file, err := os.Create(filepath.Clean(filePath))
		if err != nil {
			return fmt.Errorf("failed to create file %v for streaming: %w", filePath, err)
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}()

		if _, err := io.Copy(file, downloadStream); err != nil {
			return fmt.Errorf("failed to stream file %v to disk: %w", fileName, err)
		}
	} else {
		// Read small files into memory
		data, err := io.ReadAll(downloadStream)
		if err != nil {
			return fmt.Errorf("failed to read data from download stream: %w", err)
		}

		if err := os.WriteFile(filepath.Clean(filePath), data, 0600); err != nil {
			return fmt.Errorf("failed to write file %v to disk: %w", filePath, err)
		}
	}

	return nil
}

// Disconnect disconnects the mongo client.
func (c *Client) Disconnect(ctx context.Context) error {
	if err := c.client.Disconnect(ctx); err != nil {
		return fmt.Errorf("failed to disconnect MongoDB: %w", err)
	}
	return nil
}
