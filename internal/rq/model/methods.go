package model

// Supported HTTP methods accepted by both rq runtime specs and pm2rq conversion.
const (
	MethodGet     = "GET"
	MethodPost    = "POST"
	MethodPut     = "PUT"
	MethodPatch   = "PATCH"
	MethodDelete  = "DELETE"
	MethodHead    = "HEAD"
	MethodOptions = "OPTIONS"
)

var supportedMethods = map[string]struct{}{
	MethodGet:     {},
	MethodPost:    {},
	MethodPut:     {},
	MethodPatch:   {},
	MethodDelete:  {},
	MethodHead:    {},
	MethodOptions: {},
}

// IsSupportedMethod reports whether method is in the canonical supported set.
func IsSupportedMethod(method string) bool {
	_, ok := supportedMethods[method]
	return ok
}

// SupportedMethods returns the canonical method list in stable order.
func SupportedMethods() []string {
	return []string{
		MethodGet,
		MethodPost,
		MethodPut,
		MethodPatch,
		MethodDelete,
		MethodHead,
		MethodOptions,
	}
}
