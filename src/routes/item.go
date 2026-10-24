package routes

import (
	"context"
	"net/http"
	"skycrypt/src/lib"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

type itemTextureResolution struct {
	Texture     string `json:"texture"`
	TexturePack string `json:"texture_pack,omitempty"`
}

type ItemInput struct {
	ItemID string `path:"itemId" doc:"SkyBlock item ID or Minecraft item identifier" example:"ASPECT_OF_THE_END"`
	PacksInput
}

type ResolveInput struct {
	ItemInput
	EnabledPacksQuery string `query:"enabledPacks" doc:"Resource packs to resolve with, in place of the enabledPacks cookie."`
}

func (in *ResolveInput) packs() []string {
	if in.EnabledPacksQuery != "" {
		return lib.NormalizeEnabledPacks(parseEnabledPacks(in.EnabledPacksQuery))
	}
	return in.enabledPacks
}

func registerItem(humaAPI huma.API) {
	responses := errorResponses(humaAPI, map[int]string{
		fiber.StatusInternalServerError: "RENDER_FAILED: Item texture could not be resolved.",
	})
	responses["200"] = &huma.Response{Description: "Texture resolved successfully."}
	huma.Register(humaAPI, huma.Operation{
		OperationID: "resolveItemImage",
		Method:      fiber.MethodGet,
		Path:        "/api/item/{itemId}/resolve",
		Summary:     "Resolve an item texture",
		Description: "Returns the final texture URL and the resource pack that supplied it.",
		Tags:        []string{"Rendering"},
		Responses:   responses,
	}, func(_ context.Context, in *ResolveInput) (*bodyOutput[itemTextureResolution], error) {
		texture, err := lib.ResolveItemTexture(in.ItemID, in.packs(), false)
		if err != nil {
			return nil, NewAPIError(fiber.StatusInternalServerError, codeRenderFailed, "The item texture could not be resolved or rendered.")
		}

		return &bodyOutput[itemTextureResolution]{Body: itemTextureResolution{Texture: texture.Texture, TexturePack: texture.TexturePack}}, nil
	})

	responses = errorResponses(humaAPI, map[int]string{
		fiber.StatusInternalServerError: "RENDER_FAILED: Item image could not be rendered.",
	})
	responses["200"] = imageResponse("Image bytes returned successfully.", "image/png", "image/webp")
	responses["302"] = &huma.Response{
		Description: "Redirects to a canonical rendered asset URL.",
		Headers:     map[string]*huma.Header{"Location": {Description: "The rendered asset URL.", Schema: &huma.Schema{Type: huma.TypeString}}},
	}
	huma.Register(humaAPI, huma.Operation{
		OperationID: "renderItemImage",
		Method:      fiber.MethodGet,
		Path:        "/api/item/{itemId}",
		Summary:     "Render an item",
		Description: "Renders a SkyBlock or Minecraft item as image bytes.\n" +
			"Resource pack preferences supplied by cookie can affect which texture is rendered. Some renderer results redirect to a canonical asset URL.",
		Tags:      []string{"Rendering"},
		Responses: responses,
	}, func(_ context.Context, in *ItemInput) (*imageOutput, error) {
		texture, err := lib.ResolveItemTexture(in.ItemID, in.enabledPacks, false)
		var textureBytes []byte
		if err == nil {
			textureBytes, err = lib.RenderResolvedItemTexture(texture, false)
		}
		if err != nil {
			if redirectErr, ok := err.(lib.RedirectError); ok {
				return &imageOutput{Status: fiber.StatusFound, Location: redirectErr.URL}, nil
			}
			return nil, NewAPIError(fiber.StatusInternalServerError, codeRenderFailed, "The item texture could not be resolved or rendered.")
		}

		// Depends on the pack cookie, so browsers may keep it but shared caches may not (Vary: Cookie is set).
		// A fallback is kept briefly so the real render replaces it once it exists.
		cacheControl := "private, max-age=60"
		if lib.IsRenderedItemTexture(texture) {
			cacheControl = "private, max-age=3600"
		}
		// Rendered textures are mostly WebP; send the type the bytes actually have.
		return &imageOutput{Status: fiber.StatusOK, ContentType: http.DetectContentType(textureBytes), CacheControl: cacheControl, Body: textureBytes}, nil
	})
}
