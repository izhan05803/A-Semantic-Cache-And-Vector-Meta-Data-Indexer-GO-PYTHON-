package cache

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"semantic-cache-proxy/pb"
)

// Client is a wrapper around the gRPC CacheServiceClient
type Client struct {
	grpcClient pb.CacheServiceClient
	conn       *grpc.ClientConn
}

// NewClient establishes a gRPC connection to the Python Indexer and returns a Client
func NewClient(addr string) (*Client, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	client := pb.NewCacheServiceClient(conn)
	return &Client{
		grpcClient: client,
		conn:       conn,
	}, nil
}

// Close closes the underlying gRPC connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// CheckCache queries the Python indexer to see if there is a semantically similar prompt in the cache
func (c *Client) CheckCache(ctx context.Context, prompt string, threshold float32) (bool, string, float32, error) {
	req := &pb.CacheRequest{
		Prompt:             prompt,
		SimilarityThreshold: threshold,
	}
	resp, err := c.grpcClient.CheckCache(ctx, req)
	if err != nil {
		return false, "", 0, err
	}
	return resp.GetHit(), resp.GetResponse(), resp.GetScore(), nil
}

// UpdateCache registers a new prompt-response pair in the Python indexer
func (c *Client) UpdateCache(ctx context.Context, prompt string, response string, metadata map[string]string) (bool, error) {
	req := &pb.CacheData{
		Prompt:   prompt,
		Response: response,
		Metadata: metadata,
	}
	resp, err := c.grpcClient.UpdateCache(ctx, req)
	if err != nil {
		return false, err
	}
	return resp.GetSuccess(), nil
}
