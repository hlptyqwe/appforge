package helper

import (
	"appforge/proto/common"
)

func OkResp() *common.RespBase {
	return &common.RespBase{Code: 200, Msg: "OK"}
}

func OkWithOthers(total int64, hasNext bool, hasPrev bool, nextCursor int64, prevCursor int64) *common.RespBase {
	return &common.RespBase{
		Code:       200,
		Msg:        "OK",
		Total:      total,
		HasNext:    hasNext,
		HasPrev:    hasPrev,
		NextCursor: nextCursor,
		PrevCursor: prevCursor,
	}
}

func FailResp() *common.RespBase {
	return &common.RespBase{Code: 1, Msg: "FAIL"}
}

func FailWithCode(code int32) *common.RespBase {
	return &common.RespBase{Code: code, Msg: "FAIL"}
}

func ErrResp(code int32, msg string) *common.RespBase {
	return &common.RespBase{Code: code, Msg: msg}
}
