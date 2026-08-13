package pageutil

import (
	"appforge/common/helper"
	"appforge/proto/common"
)

func Input(page *common.PageReq) (int64, int64) {
	if page == nil {
		return 0, 10
	}
	return page.Cursor, NormalizeLimit(page.Limit)
}

func Count(page *common.PageReq) int64 {
	if page == nil {
		return 0
	}
	return page.Count
}

func Output(page *common.PageReq, limit int64) *common.PageReq {
	cursor := int64(0)
	if page != nil {
		cursor = page.Cursor
	}
	return &common.PageReq{Cursor: cursor, Limit: NormalizeLimit(limit), Count: Count(page)}
}

func Base(cursor, limit int64, size int, total int64, lastID int64) *common.RespBase {
	prevCursor := cursor
	if prevCursor < 0 {
		prevCursor = 0
	}
	hasPrev := prevCursor > 0
	hasNext := int64(size) == NormalizeLimit(limit) && lastID > 0
	nextCursor := int64(0)
	if hasNext {
		nextCursor = lastID
	}
	return helper.OkWithOthers(total, hasNext, hasPrev, nextCursor, prevCursor)
}

func TimeRangeStart(tr *common.TimeRange) int64 {
	if tr == nil {
		return 0
	}
	return tr.StartTime
}

func TimeRangeEnd(tr *common.TimeRange) int64 {
	if tr == nil {
		return 0
	}
	return tr.EndTime
}

func NormalizeLimit(limit int64) int64 {
	if limit <= 0 {
		return 10
	}
	if limit > 100 {
		return 100
	}
	return limit
}
