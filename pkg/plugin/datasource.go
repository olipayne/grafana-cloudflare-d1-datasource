package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/olipayne/grafana-cloudflare-d1-datasource/pkg/models"
)

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler interfaces. Plugin should not implement all these
// interfaces - only those which are required for a particular task.
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

// NewDatasource creates a new datasource instance.
func NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	pluginSettings, err := models.LoadPluginSettings(settings)
	if err != nil {
		return nil, fmt.Errorf("could not load plugin settings: %w", err)
	}

	return &Datasource{
		settings: pluginSettings,
	}, nil
}

// Datasource is an example datasource which can respond to data queries, reports
// its health and has streaming skills.
type Datasource struct{
	settings *models.PluginSettings
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewSampleDatasource factory function.
func (d *Datasource) Dispose() {
	// Clean up datasource instance resources.
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := d.query(ctx, req.PluginContext, q)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

// This queryModel is a simple placeholder from the original scaffold, we might not need it
// if query.JSON directly contains what we need (e.g. queryText, etc.)
// For now, we use a local qm struct inside the query method.
// type queryModel struct{}

func (d *Datasource) query(_ context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	dataResponse := backend.DataResponse{}

	var qm struct {
		QueryText string `json:"queryText"`
	}

	if err := json.Unmarshal(query.JSON, &qm); err != nil {
		dataResponse.Error = fmt.Errorf("json unmarshal query: %w", err)
		return dataResponse
	}

	if qm.QueryText == "" {
		dataResponse.Error = fmt.Errorf("empty query text")
		return dataResponse
	}

	log.DefaultLogger.Debug("Executing D1 query", "QueryText", qm.QueryText, "AccountID", d.settings.AccountID)

	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/raw",
		d.settings.AccountID, d.settings.DatabaseID)

	queryPayload := models.D1QueryRequest{SQL: qm.QueryText}
	jsonBody, err := json.Marshal(queryPayload)
	if err != nil {
		dataResponse.Error = fmt.Errorf("error marshalling D1 query payload: %w", err)
		return dataResponse
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		dataResponse.Error = fmt.Errorf("error creating HTTP request for D1: %w", err)
		return dataResponse
	}

	httpReq.Header.Set("Authorization", "Bearer "+d.settings.Secrets.APIToken)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		dataResponse.Error = fmt.Errorf("error executing D1 API request: %w", err)
		return dataResponse
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		dataResponse.Error = fmt.Errorf("error reading D1 API response body: %w", err)
		return dataResponse
	}

	if httpResp.StatusCode != http.StatusOK {
		log.DefaultLogger.Error("D1 API request failed", "status", httpResp.Status, "body", string(bodyBytes))
		dataResponse.Error = fmt.Errorf("D1 API request failed with status %s. Response: %s", httpResp.Status, string(bodyBytes))
		return dataResponse
	}

	var d1Response models.D1RawAPIResponse
	if err := json.Unmarshal(bodyBytes, &d1Response); err != nil {
		log.DefaultLogger.Error("Error unmarshalling D1 raw response", "error", err, "body", string(bodyBytes))
		dataResponse.Error = fmt.Errorf("error unmarshalling D1 API raw response: %w. Body: %s", err, string(bodyBytes))
		return dataResponse
	}

	if !d1Response.Success {
		var errorMessages string
		for _, d1Err := range d1Response.Errors {
			errorMessages += fmt.Sprintf("Code %d: %s ", d1Err.Code, d1Err.Message)
		}
		log.DefaultLogger.Error("D1 API call reported not successful", "errors", errorMessages)
		dataResponse.Error = fmt.Errorf("D1 API error: %s", errorMessages)
		return dataResponse
	}

	// Start DataFrame conversion
	// Create a new DataFrame. The RefID from the query is used to link this Frame back to the specific query panel in Grafana.
	frame := data.NewFrame(query.RefID)

	// Check if the D1 response contains any result sets or any actual results in the first result item.
	if len(d1Response.Result) == 0 || d1Response.Result[0].Results == nil || len(d1Response.Result[0].Results.Rows) == 0 {
		// Also check if there are no columns, which can happen for DDL or empty results from `SELECT`s that genuinely return no rows.
		if len(d1Response.Result) > 0 && d1Response.Result[0].Results != nil && len(d1Response.Result[0].Results.Columns) == 0 && len(d1Response.Result[0].Results.Rows) == 0 {
			// This case could be a successful DDL query (like CREATE TABLE) which returns no columns/rows
			// or a SELECT that returns no rows AND no columns (less common).
			if d1Response.Result[0].Success {
				frame.AppendNotices(data.Notice{Severity: data.NoticeSeverityInfo, Text: "Query executed successfully, no data returned (e.g., DDL statement)."})
			} else {
				// If not successful, it might be an error that didn't get caught by d1Response.Success check earlier.
				log.DefaultLogger.Debug("D1 query returned no results or an error in the result item", "QueryText", qm.QueryText)
				frame.AppendNotices(data.Notice{Severity: data.NoticeSeverityWarning, Text: "Query returned no data or an error occurred in the result processing."})
			}
		} else {
			log.DefaultLogger.Debug("D1 query returned no result rows", "QueryText", qm.QueryText)
			frame.AppendNotices(data.Notice{Severity: data.NoticeSeverityInfo, Text: "Query returned no data."}) 
		}
		dataResponse.Frames = append(dataResponse.Frames, frame)
		return dataResponse
	}

	// Get the actual query results from the D1 /raw response.
	// We assume a single SQL statement in the query, so we take the first result item.
	d1RawActualResults := d1Response.Result[0].Results
	colNames := d1RawActualResults.Columns
	d1Rows := d1RawActualResults.Rows
	rowCount := len(d1Rows)

	// If colNames is empty but we have rows, something is wrong (shouldn't happen with /raw)
	if len(colNames) == 0 && rowCount > 0 {
		dataResponse.Error = fmt.Errorf("D1 /raw response has rows but no column names")
		return dataResponse
	}

	// Determine column names and their order.
	// D1 /raw endpoint returns an ordered list of column names, so no sorting is needed.
	// This directly addresses the column ordering issue.
	// firstRow := d1Results[0] // Not needed anymore
	// colNames := make([]string, 0, len(firstRow)) // Not needed anymore
	// for k := range firstRow { // Not needed anymore
	// 	colNames = append(colNames, k)
	// }
	// sort.Strings(colNames) // REMOVED: No longer sort column names alphabetically.

	// Create data fields for the DataFrame.
	// Each field corresponds to a column in the query result, using the order from d1RawActualResults.Columns.
	for colIdx, colName := range colNames {
		// Infer the data type for the column based on the value in the first row for this column.
		// This is a simplification; a more robust system might inspect multiple rows
		// or allow user-defined type mappings, especially for types like timestamps.
		var field *data.Field
		var sampleValue interface{}
		if rowCount > 0 && colIdx < len(d1Rows[0]) {
			sampleValue = d1Rows[0][colIdx]
		}

		// Switch on the type of the sample value from the first row to create a typed Field vector.
		switch v := sampleValue.(type) {
		case float64: // JSON numbers are typically unmarshalled as float64 by encoding/json
			log.DefaultLogger.Debug("Column type inference", "column", colName, "type", "float64")
			colData := make([]*float64, rowCount)
			for i, row := range d1Rows { // Populate the slice from all rows
				if colIdx < len(row) {
					if val := row[colIdx]; val != nil {
						if fVal, fOk := val.(float64); fOk { // Type assert and assign if not nil
							colData[i] = &fVal
						}
					}
				}
			}
			field = data.NewField(colName, nil, colData)
		case string:
			log.DefaultLogger.Debug("Column type inference", "column", colName, "type", "string_from_json")
			// Define the expected D1/SQLite timestamp format
			// (YYYY-MM-DD HH:MM:SS commonly returned by SQLite CURRENT_TIMESTAMP)
			const d1TimestampLayout = "2006-01-02 15:04:05" // Go's reference time format

			// Attempt to parse string values as time.Time
			// Try D1/SQLite common format first, then RFC3339Nano as a fallback.
			parsedAsTime := false
			var errParseCheck error

			// Try parsing with d1TimestampLayout
			if _, errParseCheck = time.Parse(d1TimestampLayout, v); errParseCheck == nil {
				parsedAsTime = true
				log.DefaultLogger.Debug("Column type inference: parsed as D1 format", "column", colName, "value", v)
			} else {
				// If D1 format fails, try RFC3339Nano (existing behavior)
				if _, errParseCheck = time.Parse(time.RFC3339Nano, v); errParseCheck == nil {
					parsedAsTime = true
					log.DefaultLogger.Debug("Column type inference: parsed as RFC3339Nano", "column", colName, "value", v)
				}
			}

			if parsedAsTime {
				log.DefaultLogger.Debug("Column type inference: creating time.Time field", "column", colName)
				colData := make([]*time.Time, rowCount)
				for i, row := range d1Rows {
					if colIdx < len(row) {
						if val := row[colIdx]; val != nil {
							if sVal, sOk := val.(string); sOk {
								var tValRow time.Time
								var errParseRow error
								// Try parsing again with the determined successful layout or both
								if t, err := time.Parse(d1TimestampLayout, sVal); err == nil {
									tValRow = t
									errParseRow = nil
								} else if t, err := time.Parse(time.RFC3339Nano, sVal); err == nil {
									tValRow = t
									errParseRow = nil
								} else {
									errParseRow = err // Store the last error
								}

								if errParseRow == nil {
									colData[i] = &tValRow
								} else {
									log.DefaultLogger.Warn("Failed to parse time string in row, leaving as nil", "column", colName, "row_index", i, "value", sVal, "error", errParseRow)
								}
							}
						}
					}
				}
				field = data.NewField(colName, nil, colData)
			} else { // If not a parsable time string by any supported format, treat as a regular string.
				log.DefaultLogger.Debug("Column type inference: treating as regular string", "column", colName, "value", v)
				colData := make([]*string, rowCount)
				for i, row := range d1Rows {
					if colIdx < len(row) {
						if val := row[colIdx]; val != nil {
							if sVal, sOk := val.(string); sOk {
								colData[i] = &sVal
							}
						}
					}
				}
				field = data.NewField(colName, nil, colData)
			}
		case bool:
			log.DefaultLogger.Debug("Column type inference", "column", colName, "type", "bool")
			colData := make([]*bool, rowCount)
			for i, row := range d1Rows {
				if colIdx < len(row) {
					if val := row[colIdx]; val != nil {
						if bVal, bOk := val.(bool); bOk {
							colData[i] = &bVal
						}
					}
				}
			}
			field = data.NewField(colName, nil, colData)
		case nil: // If the sample value (from the first row for this column) is nil.
			// We need to try to infer from other rows or default to string. For now, default to string if all are nil.
			// This part of type inference could be more robust.
			log.DefaultLogger.Debug("Column type inference", "column", colName, "type", "nil_sample, defaulting to string")
			colData := make([]*string, rowCount) // Defaulting to string for nil-sampled columns
			// Attempt to populate with actual string values if present in other rows, though type is fixed by sample.
			for i, row := range d1Rows {
				if colIdx < len(row) {
					if val := row[colIdx]; val != nil {
						if sVal, sOk := val.(string); sOk {
							colData[i] = &sVal
						} else {
							// If it's not nil and not a string, convert to string representation for this default case
							tempStr := fmt.Sprintf("%v", val)
							colData[i] = &tempStr
						}
					}
				}
			}
			field = data.NewField(colName, nil, colData)

		default:
			log.DefaultLogger.Debug("Column type inference", "column", colName, "type", "unknown, defaulting to string", "actual_type", reflect.TypeOf(v))
			// For any other types, or if type inference is tricky, default to string.
			// This ensures data is at least displayed, though maybe not optimally typed.
			colData := make([]*string, rowCount)
			for i, row := range d1Rows {
				if colIdx < len(row) {
					if val := row[colIdx]; val != nil {
						tempStr := fmt.Sprintf("%v", val) // Convert value to string representation
						colData[i] = &tempStr
					}
				}
			}
			field = data.NewField(colName, nil, colData)
		}
		frame.Fields = append(frame.Fields, field)
	}

	// Append the populated frame to the response.
	dataResponse.Frames = append(dataResponse.Frames, frame)
	return dataResponse
}

// Helper function to get a pointer to a string
func ptrToString(s string) *string {
	return &s
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (d *Datasource) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	log.DefaultLogger.Info("Checking health", "AccountID", d.settings.AccountID)

	var status = backend.HealthStatusOk
	var message = "Cloudflare D1 plugin is running" // Default message, will be overridden

	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/query",
		d.settings.AccountID, d.settings.DatabaseID)

	// Basic check: ensure settings are present
	if d.settings.AccountID == "" || d.settings.DatabaseID == "" || d.settings.Secrets.APIToken == "" {
		status = backend.HealthStatusError
		// Ensure the message starts with "Health check failed:" for the e2e test
		message = "Health check failed: Account ID, Database ID, or API Token is missing in datasource configuration"
		log.DefaultLogger.Error("Health check failed: missing configuration", "AccountID", d.settings.AccountID, "DatabaseID", d.settings.DatabaseID, "APITokenSet", d.settings.Secrets.APIToken != "")
		return &backend.CheckHealthResult{
			Status:  status,
			Message: message,
		}, nil
	}

	// Prepare request body
	queryPayload := map[string]string{"sql": "SELECT 1;"}
	jsonBody, err := json.Marshal(queryPayload)
	if err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("Error marshalling query payload: %s", err.Error()),
		}, nil
	}

	// Create HTTP client and request
	httpClient := &http.Client{Timeout: 10 * time.Second}
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("Error creating HTTP request: %s", err.Error()),
		}, nil
	}

	// Set headers
	httpReq.Header.Set("Authorization", "Bearer "+d.settings.Secrets.APIToken)
	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("Error executing D1 API request: %s", err.Error()),
		}, nil
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Attempt to read body for more details, but don't fail if unreadable
		var bodyBytes []byte
		var bodyReadError error
		if resp.Body != nil {
			bodyBytes, bodyReadError = io.ReadAll(resp.Body)
		}
		message := fmt.Sprintf("D1 API request failed with status %s", resp.Status)
		if bodyReadError == nil && len(bodyBytes) > 0 {
			message = fmt.Sprintf("%s. Response: %s", message, string(bodyBytes))
		}
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: message,
		}, nil
	}

	// If we reach here, the API call was successful
	message = "Health check successful: Successfully connected to Cloudflare D1."

	return &backend.CheckHealthResult{
		Status:  status,
		Message: message,
	}, nil
}
