package logicutil

import (
	"context"
	"reflect"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	codeForbidden          int32 = 403
	codeNotFound           int32 = 404
	codeTooManyRequests    int32 = 429
	codeServiceUnavailable int32 = 100000
	codeSystemError        int32 = 100001
	codeUnauthorized       int32 = 100002
	codeBadRequest         int32 = 100003
)

func Proxy[Resp any, PReq any, PResp any](ctx context.Context, req any, call func(context.Context, *PReq, ...grpc.CallOption) (*PResp, error)) (*Resp, error) {
	protoReq := new(PReq)
	if err := copyValue(reflect.ValueOf(protoReq), reflect.ValueOf(req)); err != nil {
		return SystemErrorResp[Resp](ctx, err)
	}

	protoResp, err := call(ctx, protoReq)
	if err != nil {
		resp := new(Resp)
		code, _ := rpcErrorCodeAndMessage(err)
		if code == codeSystemError {
			logx.WithContext(ctx).Errorf("proxy rpc system error: %v", err)
		}
		if setErrorResp(resp, err) {
			return resp, nil
		}
		return SystemErrorResp[Resp](ctx, err)
	}

	resp := new(Resp)
	if err := copyValue(reflect.ValueOf(resp), reflect.ValueOf(protoResp)); err != nil {
		return SystemErrorResp[Resp](ctx, err)
	}

	return resp, nil
}

func SystemErrorResp[Resp any](ctx context.Context, err error) (*Resp, error) {
	logx.WithContext(ctx).Errorf("logic system error: %v", err)

	resp := new(Resp)
	if setSystemErrorResp(resp, err) {
		return resp, nil
	}
	return resp, nil
}

func setSystemErrorResp(resp any, err error) bool {
	v := reflect.ValueOf(resp)
	if setRespCodeAndMsg(v, codeSystemError, err.Error()) {
		return true
	}

	base, ok := findField(v, "RespBase")
	if !ok {
		base, ok = findField(v, "Base")
	}
	if !ok {
		return false
	}
	if base.Kind() == reflect.Pointer {
		if base.IsNil() {
			base.Set(reflect.New(base.Type().Elem()))
		}
		base = base.Elem()
	}
	return setRespCodeAndMsg(base, codeSystemError, err.Error())
}

func setErrorResp(resp any, err error) bool {
	code, msg := rpcErrorCodeAndMessage(err)
	v := reflect.ValueOf(resp)
	if setRespCodeAndMsg(v, code, msg) {
		return true
	}

	base, ok := findField(v, "RespBase")
	if !ok {
		base, ok = findField(v, "Base")
	}
	if !ok {
		return false
	}
	if base.Kind() == reflect.Pointer {
		if base.IsNil() {
			base.Set(reflect.New(base.Type().Elem()))
		}
		base = base.Elem()
	}
	return setRespCodeAndMsg(base, code, msg)
}

func setRespCodeAndMsg(v reflect.Value, code int32, msg string) bool {
	codeField, okCode := findField(v, "Code")
	msgField, okMsg := findField(v, "Msg")
	if !okCode || !okMsg || !codeField.CanSet() || !msgField.CanSet() {
		return false
	}
	codeField.SetInt(int64(code))
	msgField.SetString(msg)
	return true
}

func rpcErrorCodeAndMessage(err error) (int32, string) {
	st, ok := status.FromError(err)
	if !ok {
		return codeSystemError, err.Error()
	}

	code := int32(st.Code())
	if code >= 1000 {
		return code, st.Message()
	}

	switch st.Code() {
	case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange:
		return codeBadRequest, st.Message()
	case codes.Unauthenticated:
		return codeUnauthorized, st.Message()
	case codes.PermissionDenied:
		return codeForbidden, st.Message()
	case codes.NotFound:
		return codeNotFound, st.Message()
	case codes.ResourceExhausted:
		return codeTooManyRequests, st.Message()
	case codes.DeadlineExceeded, codes.Unavailable:
		return codeServiceUnavailable, st.Message()
	default:
		return codeSystemError, st.Message()
	}
}

func copyValue(dst, src reflect.Value) error {
	if !dst.IsValid() || !src.IsValid() {
		return nil
	}

	for dst.Kind() == reflect.Pointer {
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		dst = dst.Elem()
	}

	for src.Kind() == reflect.Pointer {
		if src.IsNil() {
			return nil
		}
		src = src.Elem()
	}

	if !src.IsValid() || !dst.CanSet() {
		return nil
	}

	switch dst.Kind() {
	case reflect.Struct:
		if src.Kind() != reflect.Struct {
			if src.Type().AssignableTo(dst.Type()) {
				dst.Set(src)
			} else if src.Type().ConvertibleTo(dst.Type()) {
				dst.Set(src.Convert(dst.Type()))
			}
			return nil
		}
		copyStruct(dst, src)
	case reflect.Slice:
		if src.Kind() != reflect.Slice && src.Kind() != reflect.Array {
			return nil
		}
		slice := reflect.MakeSlice(dst.Type(), src.Len(), src.Len())
		for i := 0; i < src.Len(); i++ {
			if err := copyValue(slice.Index(i), src.Index(i)); err != nil {
				return err
			}
		}
		dst.Set(slice)
	default:
		if src.Type().AssignableTo(dst.Type()) {
			dst.Set(src)
		} else if src.Type().ConvertibleTo(dst.Type()) {
			dst.Set(src.Convert(dst.Type()))
		}
	}

	return nil
}

func copyStruct(dst, src reflect.Value) {
	srcType := src.Type()
	for i := 0; i < src.NumField(); i++ {
		sf := srcType.Field(i)
		if sf.PkgPath != "" {
			continue
		}

		srcField := src.Field(i)
		if sf.Anonymous {
			_ = copyValue(dst, srcField)
			continue
		}

		if sf.Name == "PageReq" {
			setPageField(dst, src)
			continue
		}

		targetName := sf.Name
		if targetName == "Base" || targetName == "Page" {
			targetName = "RespBase"
		}

		if dstField, ok := findField(dst, targetName); ok {
			_ = copyValue(dstField, srcField)
			continue
		}

		if sf.Name == "Base" || sf.Name == "Page" {
			_ = copyValue(dst, srcField)
		}
	}

	setPageField(dst, src)
}

func setPageField(dst, src reflect.Value) {
	pageField, ok := findField(dst, "Page")
	if !ok {
		return
	}

	cursorField, okCursor := findField(src, "Cursor")
	limitField, okLimit := findField(src, "Limit")
	if !okCursor || !okLimit {
		return
	}

	if pageField.Kind() == reflect.Pointer {
		if pageField.IsNil() {
			pageField.Set(reflect.New(pageField.Type().Elem()))
		}
		pageField = pageField.Elem()
	}

	if cursorDst, ok := findField(pageField, "Cursor"); ok {
		_ = copyValue(cursorDst, cursorField)
	}
	if limitDst, ok := findField(pageField, "Limit"); ok {
		_ = copyValue(limitDst, limitField)
	}
	if countField, ok := findField(src, "Count"); ok {
		if countDst, ok := findField(pageField, "Count"); ok {
			_ = copyValue(countDst, countField)
		}
	}
}

func findField(v reflect.Value, name string) (reflect.Value, bool) {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		fieldType := t.Field(i)
		fieldValue := v.Field(i)
		if fieldType.PkgPath != "" {
			continue
		}
		if fieldType.Name == name {
			return fieldValue, true
		}
		if fieldType.Anonymous {
			if nested, ok := findField(fieldValue, name); ok {
				return nested, true
			}
		}
	}

	return reflect.Value{}, false
}
