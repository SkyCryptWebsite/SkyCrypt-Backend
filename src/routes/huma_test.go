package routes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"skycrypt/src/api"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
)

// apiKeyMiddleware answers NOT_CACHED after the handler has already written its error; the replacement must win
// outright rather than be appended to Huma's body.
func TestMiddlewareReplacesWrittenError(t *testing.T) {
	app := fiber.New()
	humaAPI := humafiber.New(app, huma.DefaultConfig("test", "1"))
	huma.Register(humaAPI, huma.Operation{
		Method: fiber.MethodGet,
		Path:   "/",
		Middlewares: huma.Middlewares{func(ctx huma.Context, next func(huma.Context)) {
			next(ctx)
			apiKeyError(humafiber.Unwrap(ctx), fiber.StatusNotFound, codeNotCached, "not cached")
		}},
	}, func(context.Context, *struct{}) (*jsonOutput, error) {
		return nil, loadError("failed", api.ErrUpstream)
	})

	response, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var got APIError
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("body %q: %v", body, err)
	}
	if response.StatusCode != fiber.StatusNotFound || got.Code != codeNotCached || response.Header.Get(fiber.HeaderContentType) != "application/problem+json" {
		t.Fatalf("status %d, body %q", response.StatusCode, body)
	}
}

func TestSpecDocumentsEveryOperation(t *testing.T) {
	spec := NewAPI(fiber.New(), "").OpenAPI()
	operations := 0
	for path, item := range spec.Paths {
		operation := item.Get
		if operation == nil {
			t.Errorf("%s: no GET operation", path)
			continue
		}
		operations++
		if operation.OperationID == "" || operation.Summary == "" {
			t.Errorf("%s: missing operation ID or summary", path)
		}
		if response := operation.Responses["200"]; response == nil || len(response.Content) == 0 {
			t.Errorf("%s: 200 response has no content", path)
		}
	}
	if operations != 21 {
		t.Errorf("documented %d operations, want 21", operations)
	}
}

func TestInvalidPlayerLookupsFailValidation(t *testing.T) {
	t.Setenv("DISABLE_SERVER_API_AUTH", "true")
	app := fiber.New()
	NewAPI(app, "")

	for _, path := range []string{"/api/uuid/ab", "/api/uuid/seventeen_chars_x", "/api/username/notauuid", "/api/username/4855C53EE4FB4100997600A92FC50984"} {
		response, err := app.Test(httptest.NewRequest("GET", path, nil))
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		var got APIError
		if err := json.Unmarshal(body, &got); err != nil || response.StatusCode != fiber.StatusUnprocessableEntity || got.Code != "INVALID_REQUEST" {
			t.Errorf("%s: status %d, body %s", path, response.StatusCode, body)
		}
	}
}

// Handlers find whether a key may reach Hypixel in the request state apiKeyMiddleware adds, so a middleware that
// passes the original context on would quietly let cache-only keys fetch fresh data.
func TestAPIKeyMiddlewarePassesRequestState(t *testing.T) {
	t.Setenv("SERVER_API_TOKEN", "test-token")
	app := fiber.New()
	humaAPI := humafiber.New(app, huma.DefaultConfig("test", "1"))
	huma.Register(humaAPI, requireAPIKey(humaAPI, huma.Operation{Method: fiber.MethodGet, Path: "/"}),
		func(ctx context.Context, _ *struct{}) (*bodyOutput[bool], error) {
			// Marks the request state, so ServedStale reports whether there is one.
			api.SkipCaching(ctx)
			return &bodyOutput[bool]{Body: api.ServedStale(ctx)}, nil
		})

	for name, setup := range map[string]func(*http.Request){
		"auth disabled": func(*http.Request) { t.Setenv("DISABLE_SERVER_API_AUTH", "true") },
		"frontend token": func(request *http.Request) {
			t.Setenv("DISABLE_SERVER_API_AUTH", "false")
			request.Header.Set("X-API-Token", "test-token")
		},
	} {
		request := httptest.NewRequest("GET", "/", nil)
		setup(request)
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != fiber.StatusOK || strings.TrimSpace(string(body)) != "true" {
			t.Errorf("%s: status %d, body %s; want the handler to see the request state", name, response.StatusCode, body)
		}
	}
}
