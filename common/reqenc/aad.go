package reqenc

import (
	"fmt"
	"net/http"
	"strings"
)

func BuildAAD(location Location, keyID, timestamp, nonce, method, bindingTarget string) []byte {
	return []byte(fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n%s",
		Version, location, keyID, timestamp, nonce, strings.ToUpper(method), bindingTarget))
}

func BindingTarget(r *http.Request, location Location, pathTemplate string) string {
	switch location {
	case LocationQuery:
		return r.URL.Path + "?data"
	case LocationPath:
		return pathTemplate
	default:
		if r.URL.RawQuery == "" {
			return r.URL.Path
		}
		return r.URL.Path + "?" + r.URL.RawQuery
	}
}
