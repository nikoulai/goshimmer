package jsonmodels

import "time"

// PastconeRequest holds the message id to query.
type PastconeRequest struct {
	ID string `json:"id"`
}

// PastconeResponse is the HTTP response containing the number of messages in the past cone and if all messages of the past cone
// exist on the node.
type PastconeResponse struct {
	Exist        bool   `json:"exist,omitempty"`
	PastConeSize int    `json:"pastConeSize,omitempty"`
	Error        string `json:"error,omitempty"`
}

// MissingResponse is the HTTP response containing all the missing messages and their count.
type MissingResponse struct {
	IDs   []string `json:"ids,omitempty"`
	Count int      `json:"count,omitempty"`
}

// MissingAvailableResponse is a map of messageIDs with the peers that have such message.
type MissingAvailableResponse struct {
	Availability map[string][]string `json:"msgavailability,omitempty"`
	Count        int                 `json:"count"`
}

// OrphanageResponse is a struct providing response for an orphanage diagnostic API
type OrphanageResponse struct {
	CreatorNodeID string         `json:"creatorNodeId"`
	WalkStartTime int64          `json:"walkStartTime"`
	MaxParentAge  int64          `json:"maxParentAge"`
	OrphansByNode map[string]int `json:"orphansByNode,omitempty"`
	IssuedByNode  map[string]int `json:"issuedByNode,omitempty"`
}

// NewOrphanageResponse creates a response object for OrphanageResponse json model
func NewOrphanageResponse(nodeId string, startTime time.Time, maxAge time.Duration, orphansByNode map[string]int, issuedByNode map[string]int) *OrphanageResponse {
	return &OrphanageResponse{
		CreatorNodeID: nodeId,
		WalkStartTime: startTime.UnixNano(),
		MaxParentAge:  maxAge.Nanoseconds(),
		OrphansByNode: orphansByNode,
		IssuedByNode:  issuedByNode,
	}
}
