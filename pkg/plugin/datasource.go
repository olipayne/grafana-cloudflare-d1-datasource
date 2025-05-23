package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
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

	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/query",
		d.settings.AccountID, d.settings.DatabaseID)

	queryPayload := models.D1QueryRequest{SQL: qm.QueryText}
	jsonBody, err := json.Marshal(queryPayload)
	if err != nil {
		dataResponse.Error = fmt.Errorf("error marshalling D1 query payload: %w", err)
		return dataResponse
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
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

	var d1Response models.D1APIResponse
	if err := json.Unmarshal(bodyBytes, &d1Response); err != nil {
		log.DefaultLogger.Error("Error unmarshalling D1 response", "error", err, "body", string(bodyBytes))
		dataResponse.Error = fmt.Errorf("error unmarshalling D1 API response: %w. Body: %s", err, string(bodyBytes))
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

	// Check if the D1 response contains any result sets or any rows in the first result set.
	if len(d1Response.Result) == 0 || len(d1Response.Result[0].Results) == 0 {
		log.DefaultLogger.Debug("D1 query returned no results", "QueryText", qm.QueryText)
		// If no data, append a notice to the frame and return. Grafana will display this notice.
		frame.AppendNotices(data.Notice{Severity: data.NoticeSeverityInfo, Text: "Query returned no data."}) 
		dataResponse.Frames = append(dataResponse.Frames, frame)
		return dataResponse
	}

	// Get the actual query results (rows) from the D1 response.
	// We assume a single result set from D1, as batch queries are not explicitly handled here.
	d1Results := d1Response.Result[0].Results
	rowCount := len(d1Results)

	// Determine column names and their order.
	// D1 returns results as []map[string]interface{}, where keys in the map are column names.
	// Map iteration order in Go is not guaranteed. To ensure consistent column order in Grafana,
	// we extract all column names from the first row and then sort them alphabetically.
	// A more ideal solution would be if D1 provided ordered column metadata.
	firstRow := d1Results[0]
	colNames := make([]string, 0, len(firstRow))
	for k := range firstRow {
		colNames = append(colNames, k)
	}
	sort.Strings(colNames) // Sort column names alphabetically for consistent order.

	// Create data fields for the DataFrame.
	// Each field corresponds to a column in the query result.
	for _, colName := range colNames {
		// Infer the data type for the column based on the value in the first row.
		// This is a simplification; a more robust system might inspect multiple rows
		// or allow user-defined type mappings, especially for types like timestamps.
		var field *data.Field
		sampleValue := firstRow[colName]

		// Switch on the type of the sample value from the first row to create a typed Field vector.
		switch v := sampleValue.(type) {
		case float64: // JSON numbers are typically unmarshalled as float64 by encoding/json
			log.DefaultLogger.Debug("Column type inference", "column", colName, "type", "float64")
			// Create a slice of nullable float64s to hold data for this column.
			colData := make([]*float64, rowCount)
			for i, row := range d1Results { // Populate the slice from all rows
				if val, ok := row[colName]; ok && val != nil {
					if fVal, fOk := val.(float64); fOk { // Type assert and assign if not nil
						colData[i] = &fVal
					}
				}
			}
			field = data.NewField(colName, nil, colData)
		case string:
			log.DefaultLogger.Debug("Column type inference", "column", colName, "type", "string")
			// Attempt to parse string values as time.Time if they match RFC3339Nano.
			// This is a common timestamp format. More sophisticated parsing or user configuration
			// might be needed for other timestamp formats D1 might return.
			if _, e := time.Parse(time.RFC3339Nano, v); e == nil { // Check if first value parses as time
				log.DefaultLogger.Debug("Column type inference", "column", colName, "detected_type", "time.Time from string")
				colData := make([]*time.Time, rowCount)
				for i, row := range d1Results {
					if val, ok := row[colName]; ok && val != nil {
						if sVal, sOk := val.(string); sOk {
							if tVal, errT := time.Parse(time.RFC3339Nano, sVal); errT == nil {
								colData[i] = &tVal
							}
						}
					}
				}
				field = data.NewField(colName, nil, colData)
			} else { // If not a parsable time string, treat as a regular string.
				colData := make([]*string, rowCount)
				for i, row := range d1Results {
					if val, ok := row[colName]; ok && val != nil {
						if sVal, sOk := val.(string); sOk {
							colData[i] = &sVal
						}
					}
				}
				field = data.NewField(colName, nil, colData)
			}
		case bool:
			log.DefaultLogger.Debug("Column type inference", "column", colName, "type", "bool")
			colData := make([]*bool, rowCount)
			for i, row := range d1Results {
				if val, ok := row[colName]; ok && val != nil {
					if bVal, bOk := val.(bool); bOk {
						colData[i] = &bVal
					}
				}
			}
			field = data.NewField(colName, nil, colData)
		case nil: // If the sample value (from the first row) is nil.
			// We cannot infer the type. Default to string for this column.
			// This could be an issue if subsequent rows have a non-string type (e.g. number).
			log.DefaultLogger.Debug("Column type inference", "column", colName, "type", "nil in first row, defaulting to string")
			colData := make([]*string, rowCount)
			for i, row := range d1Results {
				if val, ok := row[colName]; ok && val != nil { // Process non-nil values if they appear later
					// Attempt to convert to string. This might not be ideal if other types appear.
					colData[i] = ptrToString(fmt.Sprintf("%v", val))
				} 
			}
			field = data.NewField(colName, nil, colData)
		default: // Fallback for any other types not explicitly handled.
			// Convert to string. This ensures data is displayed but might lose original typing.
			log.DefaultLogger.Debug("Column type inference", "column", colName, "type", "unknown, defaulting to string", "actual_type", reflect.TypeOf(sampleValue))
			colData := make([]*string, rowCount)
			for i, row := range d1Results {
				if val, ok := row[colName]; ok && val != nil {
					colData[i] = ptrToString(fmt.Sprintf("%v", val))
				}
			}
			field = data.NewField(colName, nil, colData)
		}
		// Add the newly created and populated field to the DataFrame.
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
	if d.settings == nil || d.settings.Secrets == nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Plugin settings not loaded correctly",
		}, nil
	}

	if d.settings.AccountID == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Account ID is missing",
		}, nil
	}

	if d.settings.DatabaseID == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Database ID is missing",
		}, nil
	}

	if d.settings.Secrets.APIToken == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "API Token is missing",
		}, nil
	}

	// Construct Cloudflare D1 API URL
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/query",
		d.settings.AccountID, d.settings.DatabaseID)

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

	// Optionally, you could parse the response to ensure it's valid if SELECT 1 returns specific data.
	// For now, a 200 OK is sufficient for a health check.

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Successfully connected to Cloudflare D1 and executed test query.",
	}, nil
}
