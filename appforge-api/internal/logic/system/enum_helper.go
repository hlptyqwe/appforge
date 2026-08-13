package system

import (
	"strings"

	"appforge/proto/common"
	"appforge/proto/system"
)

func toCommonStatus(v int64) common.Enable {
	return common.Enable(v)
}

func fromCommonStatus(v common.Enable) int64 {
	return int64(v)
}

func toMenuType(v int64) system.MenuType {
	return system.MenuType(v)
}

func fromMenuType(v system.MenuType) int64 {
	return int64(v)
}

func toVisibleStatus(v int64) common.Switch {
	return common.Switch(v)
}

func fromVisibleStatus(v common.Switch) int64 {
	return int64(v)
}

func toRequestMethod(v string) system.RequestMethod {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "GET":
		return system.RequestMethod_REQUEST_METHOD_GET
	case "POST":
		return system.RequestMethod_REQUEST_METHOD_POST
	case "PUT":
		return system.RequestMethod_REQUEST_METHOD_PUT
	case "DELETE":
		return system.RequestMethod_REQUEST_METHOD_DELETE
	default:
		return system.RequestMethod_REQUEST_METHOD_UNKNOWN
	}
}

func fromRequestMethod(v system.RequestMethod) string {
	switch v {
	case system.RequestMethod_REQUEST_METHOD_GET:
		return "GET"
	case system.RequestMethod_REQUEST_METHOD_POST:
		return "POST"
	case system.RequestMethod_REQUEST_METHOD_PUT:
		return "PUT"
	case system.RequestMethod_REQUEST_METHOD_DELETE:
		return "DELETE"
	default:
		return ""
	}
}
