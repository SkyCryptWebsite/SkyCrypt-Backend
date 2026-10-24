package routes

import (
	"context"
	"errors"
	"net/http"
	"skycrypt/src/lib"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

// notFoundImageCacheControl lets clients and the CDN absorb repeats of a bad image URL without pinning the miss for long.
const notFoundImageCacheControl = "public, max-age=300"

// imageOutput is a rendered image or a redirect to one. Status must always be set.
type imageOutput struct {
	Status       int
	ContentType  string `header:"Content-Type" hidden:"true"`
	CacheControl string `header:"Cache-Control"`
	// Documented by hand on the 302 response, the only one that sets it.
	Location string `header:"Location" hidden:"true"`
	Body     []byte
}

// imageResponse documents a response with image bytes.
func imageResponse(description string, contentTypes ...string) *huma.Response {
	content := map[string]*huma.MediaType{}
	for _, contentType := range contentTypes {
		content[contentType] = &huma.MediaType{Schema: &huma.Schema{Type: huma.TypeString, Format: "binary"}}
	}
	return &huma.Response{Description: description, Content: content}
}

// renderedImage answers with the PNG a renderer made, or with the error for one it couldn't make.
func renderedImage(image []byte, err error) (*imageOutput, error) {
	if errors.Is(err, lib.ErrTextureNotFound) {
		return nil, huma.ErrorWithHeaders(
			NewAPIError(fiber.StatusNotFound, codeTextureNotFound, "No texture matches this request."),
			http.Header{fiber.HeaderCacheControl: {notFoundImageCacheControl}},
		)
	}
	if err != nil {
		return nil, NewAPIError(fiber.StatusInternalServerError, codeRenderFailed, "The image could not be rendered.")
	}
	return &imageOutput{Status: fiber.StatusOK, ContentType: "image/png", CacheControl: renderedImageCacheControl, Body: image}, nil
}

func registerHead(humaAPI huma.API) {
	responses := errorResponses(humaAPI, map[int]string{
		fiber.StatusNotFound:            "TEXTURE_NOT_FOUND: Texture identifier is malformed or unknown.",
		fiber.StatusInternalServerError: "RENDER_FAILED: Head image could not be rendered.",
	})
	responses["200"] = imageResponse("PNG image bytes returned successfully.", "image/png")
	huma.Register(humaAPI, huma.Operation{
		OperationID: "renderHeadImage",
		Method:      fiber.MethodGet,
		Path:        "/api/head/{textureId}",
		Summary:     "Render a player head",
		Description: "Renders a Minecraft player head texture as PNG bytes.",
		Tags:        []string{"Rendering"},
		Responses:   responses,
	}, func(_ context.Context, in *struct {
		TextureID string `path:"textureId" doc:"Minecraft skin texture identifier" example:"93a1b830399ab432a5178fdaf3939b24bf25c724a66be947296c503352bc380d"`
	}) (*imageOutput, error) {
		return renderedImage(lib.RenderHead(in.TextureID))
	})
}
