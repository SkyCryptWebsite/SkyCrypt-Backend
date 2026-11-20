package api

import (
	"github.com/bytedance/sonic"

	skycrypttypes "github.com/SkyCryptWebsite/SkyCrypt-Types"
)

// hypixelJSON decodes Hypixel response bodies. A profiles body is a couple of MB and decoding it is a large part of
// a build; sonic decodes it about three times faster than jsoniter, whose string reader falls back to a
// byte-by-byte path for every inventory, because Hypixel ends each base64 string with an escaped "=" (=).
// ConfigStd keeps encoding/json's behaviour: strings are copied out of the body and validated.
var hypixelJSON = sonic.ConfigStd

// The custom UnmarshalJSON methods in SkyCrypt-Types (bestiary, loadouts, skill trees, toolkits and more) decode
// their contents with encoding/json unless told otherwise; in production that reflection path was about a fifth of
// the time and allocation of decoding a profile.
func init() {
	skycrypttypes.JSONUnmarshal = hypixelJSON.Unmarshal
}
