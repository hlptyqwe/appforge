package logicutil

import (
	"sort"

	"appforge/admin-api/internal/types"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func Option(value int32, code string) types.OptionsItem {
	item := types.OptionsItem{
		Value: value,
		Code:  code,
	}
	return item
}

func Group(key, label string, options ...types.OptionsItem) types.OptionsGroup {
	return types.OptionsGroup{
		Key:     key,
		Label:   label,
		Options: options,
	}
}

func EnumGroup(key, label string, desc protoreflect.EnumDescriptor, skipValues ...int32) types.OptionsGroup {
	return Group(key, label, EnumOptions(desc, skipValues...)...)
}

func EnumOptions(desc protoreflect.EnumDescriptor, skipValues ...int32) []types.OptionsItem {
	skip := make(map[int32]struct{}, len(skipValues))
	for _, v := range skipValues {
		skip[v] = struct{}{}
	}

	values := make([]int, 0, desc.Values().Len())
	names := make(map[int32]string, desc.Values().Len())
	for i := 0; i < desc.Values().Len(); i++ {
		value := desc.Values().Get(i)
		number := int32(value.Number())
		if _, ok := skip[number]; ok {
			continue
		}
		values = append(values, int(number))
		names[number] = string(value.Name())
	}
	sort.Ints(values)

	options := make([]types.OptionsItem, 0, len(values))
	for _, raw := range values {
		value := int32(raw)
		options = append(options, Option(value, names[value]))
	}

	return options
}
