package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"
)

type Args map[string]any

func GetArgs(input []byte, params Params) (Args, error) {
	result := Args{}

	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()

	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}

	allParamKeys := params.Keys()
	seenKeys := []string{}
	unknownKeys := []string{}

	for key := range result {
		seenKeys = append(seenKeys, key)

		if !slices.Contains(allParamKeys, key) {
			unknownKeys = append(unknownKeys, key)
		}
	}

	if len(unknownKeys) > 0 {
		return nil, fmt.Errorf("got unknown keys: %s", strings.Join(unknownKeys, ", "))
	}

	missingKeys := []string{}

	for _, key := range params.RequiredKeys() {
		if !slices.Contains(seenKeys, key) {
			missingKeys = append(missingKeys, key)
		}
	}

	if len(missingKeys) > 0 {
		return nil, fmt.Errorf("missing required keys: %s", strings.Join(missingKeys, ", "))
	}

	if err := result.checkTypes(params); err != nil {
		return nil, err
	}

	return result, nil
}

func (a Args) String() string {
	resultBuilder := &strings.Builder{}
	for key, val := range a {
		fmt.Fprintf(resultBuilder, "%s=%v, ", key, val)
	}

	result := strings.TrimSuffix(resultBuilder.String(), ", ")

	return result
}

func (a Args) checkTypes(params Params) error { //nolint:cyclop
	wrongTypeKeys := []string{}

	for key, val := range a {
		param := params.ByKey(key)
		rightType := true

		switch param.Type {
		case ParamTypeBoolean:
			_, rightType = val.(bool)
		case ParamTypeNumber:
			_, isNumber := val.(json.Number)
			_, isInt := val.(int64)
			_, isFloat := val.(float64)
			rightType = isNumber || isInt || isFloat
		case ParamTypeString:
			_, rightType = val.(string)
		case ParamTypeArray:
			typ := reflect.TypeOf(val)
			rightType = (typ.Kind() == reflect.Array) || (typ.Kind() == reflect.Slice)
			// TODO: validate ItemType
		case ParamTypeObject:
			typ := reflect.TypeOf(val)
			rightType = typ.Kind() == reflect.Map
		case ParamTypeNull:
			rightType = val == nil
		}

		if !rightType {
			wrongTypeKeys = append(wrongTypeKeys, fmt.Sprintf("%s (expecting %s)", key, param.Type))
		}
	}

	if len(wrongTypeKeys) > 0 {
		return fmt.Errorf("keys with wrong types: %s", strings.Join(wrongTypeKeys, ", "))
	}

	return nil
}

func (a Args) GetString(key string) *string {
	val, hasKey := a[key]
	if !hasKey {
		return nil
	}

	strVal, isStr := val.(string)
	if !isStr {
		return nil
	}

	return &strVal
}

func (a Args) GetInt64(key string) *int64 {
	val, hasKey := a[key]
	if !hasKey {
		return nil
	}

	numVal, isNumber := val.(json.Number)
	if isNumber {
		intVal, err := numVal.Int64()
		if err != nil {
			return nil
		}

		return &intVal
	}

	intVal, isInt := val.(int64)
	if !isInt {
		return nil
	}

	return &intVal
}

func (a Args) GetFloat64(key string) *float64 {
	val, hasKey := a[key]
	if !hasKey {
		return nil
	}

	numVal, isNumber := val.(json.Number)
	if isNumber {
		floatVal, err := numVal.Float64()
		if err != nil {
			return nil
		}

		return &floatVal
	}

	floatVal, isFloat := val.(float64)
	if !isFloat {
		return nil
	}

	return &floatVal
}

func (a Args) GetBool(key string) *bool {
	val, hasKey := a[key]
	if !hasKey {
		return nil
	}

	boolVal, isBool := val.(bool)
	if !isBool {
		return nil
	}

	return &boolVal
}

func (a Args) GetStringSlice(key string) []string {
	val, hasKey := a[key]
	if !hasKey {
		return nil
	}

	sliceVal, isSlice := val.([]any)
	if !isSlice {
		return nil
	}

	stringSlice := make([]string, len(sliceVal))

	for itemNum, rawVal := range sliceVal {
		strVal, ok := rawVal.(string)
		// be brutal - no non-strings allowed
		if !ok {
			return nil
		}

		stringSlice[itemNum] = strVal
	}

	return stringSlice
}

func (a Args) LogValue() slog.Value {
	attrs := []slog.Attr{}

	for key, val := range a {
		if boolVal := a.GetBool(key); boolVal != nil {
			attrs = append(attrs, slog.Bool(key, *boolVal))
		} else if intVal := a.GetInt64(key); intVal != nil {
			attrs = append(attrs, slog.Int64(key, *intVal))
		} else if floatVal := a.GetFloat64(key); floatVal != nil {
			attrs = append(attrs, slog.Float64(key, *floatVal))
		} else if stringVal := a.GetString(key); stringVal != nil {
			attrs = append(attrs, slog.String(key, *stringVal))
		} else {
			attrs = append(attrs, slog.Any(key, val))
		}
	}

	return slog.GroupValue(attrs...)
}
