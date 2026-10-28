package routes

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
)

// NewAPI registers every endpoint on app. Huma builds the OpenAPI spec from the operations and from each handler's
// input and output types, so the spec can't drift from the code.
func init() {
	huma.NewError = newHumaError
}

func NewAPI(app *fiber.App, description string) huma.API {
	config := huma.DefaultConfig("SkyCrypt API", "3.0")
	config.Info.Description = description
	config.Servers = []*huma.Server{
		{URL: "https://api.shiiyu.moe", Description: "Production"},
		{URL: "https://beta-api.shiiyu.moe", Description: "Beta, built from the dev branch"},
	}
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		apiKeyScheme: {Type: "apiKey", In: "header", Name: "x-api-key", Description: "SkyCrypt API key, starting with `skycrypt_`."},
	}
	// SetupRoutes serves the spec and the API reference page itself.
	config.OpenAPIPath, config.DocsPath, config.SchemasPath = "", "", ""
	// No $schema field and Link header added to bodies.
	config.CreateHooks = nil
	// Responses may leave out any field, so none is documented as required (path parameters still are).
	config.FieldsOptionalByDefault = true

	api := humafiber.New(app, config)
	registerSource(api)
	registerPlayers(api)
	registerStats(api)
	registerPlayerStats(api)
	registerNetworth(api)
	registerCombined(api)
	registerInventory(api)
	registerInventorySearch(api)
	registerGarden(api)
	registerEmbed(api)
	registerItem(api)
	registerHead(api)
	registerPotion(api)
	registerLeather(api)
	registerStatsConstants(api)
	registerEnchantments(api)
	registerResourcePacks(api)
	registerLeaderboards(api)
	// Huma adds this response to operations without documented errors, with no example of its own.
	for _, item := range api.OpenAPI().Paths {
		if response := item.Get.Responses["default"]; response != nil {
			response.Content["application/problem+json"].Example = InternalError()
		}
	}
	return api
}

// APIError is Huma's error format, an RFC 9457 problem detail sent as application/problem+json, plus a code that
// clients can branch on without parsing the detail.
type APIError struct {
	huma.ErrorModel
	Code string `json:"code,omitempty" example:"NOT_CACHED" doc:"Machine-readable error code, such as PLAYER_NOT_FOUND, NOT_CACHED, or RATE_LIMITED."`
}

// NewAPIError is a failed request with its status, code, and a human-readable detail.
func NewAPIError(status int, code string, detail string) *APIError {
	return &APIError{ErrorModel: huma.ErrorModel{Status: status, Title: http.StatusText(status), Detail: detail}, Code: code}
}

// InternalError is the APIError for an unexpected failure, such as a recovered panic.
func InternalError() *APIError {
	return NewAPIError(http.StatusInternalServerError, codeInternalError, "Internal Server Error")
}

// newHumaError builds the errors Huma raises itself, such as a request that fails validation.
func newHumaError(status int, msg string, errs ...error) huma.StatusError {
	if status >= http.StatusInternalServerError {
		// Unexpected errors carry internal details, so they are logged rather than sent.
		for _, err := range errs {
			fmt.Printf("[API] %s: %v\n", msg, err)
		}
		return NewAPIError(status, codeInternalError, msg)
	}
	apiErr := NewAPIError(status, codeInvalidRequest, msg)
	for _, err := range errs {
		if err != nil {
			apiErr.Add(err)
		}
	}
	return apiErr
}

// jsonOutput is a response that is already JSON: a cached body, or one built by sharedResponse.
type jsonOutput struct {
	ContentType  string `header:"Content-Type" hidden:"true"`
	CacheControl string `header:"Cache-Control"`
	Cache        string `header:"X-SkyCrypt-Backend-Cache" doc:"redis: served from the response cache. miss: built now, then cached. bypass: built now and not cached."`
	Body         []byte
}

// jsonResponse documents the body of a jsonOutput, whose Go type ([]byte) says nothing about the JSON in it.
func jsonResponse[T any](api huma.API, description string) *huma.Response {
	schema := api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[T](), true, "")
	return &huma.Response{
		Description: description,
		Content:     map[string]*huma.MediaType{"application/json": {Schema: schema}},
	}
}

// documentedCode reads the codes and detail out of an error description written as "CODE: what happened" or
// "CODE or OTHER_CODE: what happened".
var documentedCode = regexp.MustCompile(`^([A-Z_]+)(?:,? (?:or )?[A-Z_]+)*: ([^.]+\.?)`)

// errorResponse documents an error status answered with an APIError. Its example comes from the description;
// without one, docs fall back to the placeholder examples on Huma's ErrorModel fields.
func errorResponse(api huma.API, status int, description string) *huma.Response {
	schema := api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[APIError](), true, "")
	media := &huma.MediaType{Schema: schema}
	if match := documentedCode.FindStringSubmatch(description); match != nil {
		detail := strings.ToUpper(match[2][:1]) + match[2][1:]
		media.Example = NewAPIError(status, match[1], detail)
	}
	return &huma.Response{
		Description: description,
		Content:     map[string]*huma.MediaType{"application/problem+json": media},
	}
}

// errorResponses documents several error statuses, each answered with an APIError.
func errorResponses(api huma.API, responses map[int]string) map[string]*huma.Response {
	documented := make(map[string]*huma.Response, len(responses))
	for status, description := range responses {
		documented[strconv.Itoa(status)] = errorResponse(api, status, description)
	}
	return documented
}
