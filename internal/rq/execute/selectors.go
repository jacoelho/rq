package execute

import "github.com/jacoelho/rq/internal/rq/capture"

type selectorContext struct {
	data any
	err  error
}

func selectorContextFromBody(body []byte, enabled bool) selectorContext {
	if !enabled {
		return selectorContext{}
	}

	data, err := capture.ParseJSONBody(body)
	return selectorContext{
		data: data,
		err:  err,
	}
}

func selectorContextFromData(enabled bool, data any, err error) selectorContext {
	if !enabled {
		return selectorContext{}
	}

	return selectorContext{
		data: data,
		err:  err,
	}
}
