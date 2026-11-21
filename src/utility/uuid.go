package utility

// IsUUID reports whether uuid is 32 lowercase hex characters (no dashes). It runs several times per request, so it
// checks bytes instead of compiling a regex each call.
func IsUUID(uuid string) bool {
	if len(uuid) != 32 {
		return false
	}
	for i := 0; i < len(uuid); i++ {
		c := uuid[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
