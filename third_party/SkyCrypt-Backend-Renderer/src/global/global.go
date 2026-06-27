package global

import (
	"encoding/base64"
	"net/http"

	jsoniter "github.com/json-iterator/go"
)

var HTTP_CLIENT = http.Client{}

var JSON = jsoniter.ConfigCompatibleWithStandardLibrary

var Base64 = base64.StdEncoding


