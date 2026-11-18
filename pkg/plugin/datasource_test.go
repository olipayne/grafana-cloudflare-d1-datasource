package plugin

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/olipayne/grafana-cloudflare-d1-datasource/pkg/models"
)

func TestQueryData(t *testing.T) {
	ds := Datasource{}

	resp, err := ds.QueryData(
		context.Background(),
		&backend.QueryDataRequest{
			Queries: []backend.DataQuery{
				{RefID: "A"},
			},
		},
	)
	if err != nil {
		t.Error(err)
	}

	if len(resp.Responses) != 1 {
		t.Fatal("QueryData must return a response")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestD1RequestCancellation(t *testing.T) {
	ds := Datasource{settings: &models.PluginSettings{
		AccountID: "account", DatabaseID: "database",
		Secrets: &models.SecretPluginSettings{APIToken: "test"},
	}}
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })

	// Cancel after the outbound request reaches the transport, as when a user
	// navigates away while Grafana is waiting for D1.
	mockRequest := func(t *testing.T) context.Context {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			cancel()
			if err := req.Context().Err(); err != nil {
				return nil, err
			}
			return nil, errors.New("D1 request remained active after Grafana canceled it")
		})
		return ctx
	}

	t.Run("query", func(t *testing.T) {
		resp, err := ds.QueryData(mockRequest(t), &backend.QueryDataRequest{
			Queries: []backend.DataQuery{{RefID: "A", JSON: []byte(`{"queryText":"SELECT 1"}`)}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := resp.Responses["A"].Error; !errors.Is(err, context.Canceled) {
			t.Fatalf("expected canceled query, got %v", err)
		}
	})

	t.Run("health check", func(t *testing.T) {
		result, err := ds.CheckHealth(mockRequest(t), &backend.CheckHealthRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != backend.HealthStatusError || !strings.Contains(result.Message, context.Canceled.Error()) {
			t.Fatalf("expected canceled health check, got %+v", result)
		}
	})
}
