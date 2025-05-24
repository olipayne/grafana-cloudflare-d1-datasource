package models

// D1QueryRequest is the payload for a D1 query.
// We don't strictly need this if we always send `{"sql": "..."}` directly,
// but it's good practice to define request structs as well.
type D1QueryRequest struct {
	SQL string `json:"sql"`
}

// D1SuccessResult represents the actual query results and metadata from a successful D1 query.
type D1SuccessResult struct {
	Results []map[string]interface{} `json:"results"` // Array of row objects (map of colName:value)
	Meta    D1Meta                   `json:"meta"`
	Success bool                     `json:"success"`
}

// D1Meta contains metadata about the D1 query execution.
type D1Meta struct {
	ServedBy    string  `json:"served_by"`
	Duration    float64 `json:"duration"`
	Changes     int     `json:"changes"`
	LastRowID   int     `json:"last_row_id"` // D1 docs show last_row_id, but results often have it as 0 if not an INSERT
	ChangedDB   bool    `json:"changed_db"`
	SizeAfter   int     `json:"size_after"`
	RowsRead    int     `json:"rows_read"`
	RowsWritten int     `json:"rows_written"`
}

// D1APIResponse is the top-level structure for a D1 API response.
// It contains an array of D1SuccessResult for batched queries (though we'll likely send one query at a time).
type D1APIResponse struct {
	Result   []D1SuccessResult `json:"result"`
	Success  bool              `json:"success"`
	Errors   []D1Error         `json:"errors"`   // D1 API error objects
	Messages []D1Message       `json:"messages"` // D1 API message objects
}

// D1Error represents an error object from the D1 API.
type D1Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// D1Message represents a message object from the D1 API.
type D1Message struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// D1RawQueryActualResult represents the 'results' part of a successful D1 /raw query.
// It contains an ordered list of column names and rows as arrays of values.
type D1RawQueryActualResult struct {
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
}

// D1RawResultItem represents one item in the 'result' array from a D1 /raw query.
// This structure holds the actual query results (columns and rows) and metadata.
type D1RawResultItem struct {
	Results *D1RawQueryActualResult `json:"results,omitempty"` // Pointer to handle DDL statements that don't return rows/columns
	Meta    D1Meta                  `json:"meta"`
	Success bool                    `json:"success"`
}

// D1RawAPIResponse is the top-level structure for a D1 /raw API response.
// This is similar to D1APIResponse but tailored for the /raw endpoint's structure.
type D1RawAPIResponse struct {
	Result   []D1RawResultItem `json:"result"`
	Success  bool              `json:"success"`
	Errors   []D1Error         `json:"errors"`
	Messages []D1Message       `json:"messages"`
} 