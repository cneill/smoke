package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strconv"
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
		return nil, fmt.Errorf("%w: %s", ErrUnknownKeys, strings.Join(unknownKeys, ", "))
	}

	missingKeys := []string{}

	for _, key := range params.RequiredKeys() {
		if !slices.Contains(seenKeys, key) {
			missingKeys = append(missingKeys, key)
		}
	}

	if len(missingKeys) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrMissingKeys, strings.Join(missingKeys, ", "))
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

// TODO: allow for e.g. "1" for number, "f" for bool?
func (a Args) checkTypes(params Params) error {
	wrongTypeKeys := []string{}

	for key, value := range a {
		param := params.ByKey(key)
		if !a.rightType(param, value) {
			wrongTypeKeys = append(wrongTypeKeys, fmt.Sprintf("%s (expecting %s)", key, param.Type))
		}
	}

	if len(wrongTypeKeys) > 0 {
		return fmt.Errorf("%w: %s", ErrWrongTypeKeys, strings.Join(wrongTypeKeys, ", "))
	}

	return nil
}

func (a Args) rightType(param *Param, value any) bool { //nolint:cyclop
	rightType := true
	typ := reflect.TypeOf(value)

	switch param.Type {
	case ParamTypeBoolean:
		_, rightType = value.(bool)
	case ParamTypeNumber:
		_, isNumber := value.(json.Number)
		_, isInt := value.(int)
		_, isInt64 := value.(int64)
		_, isFloat := value.(float64)
		rightType = isNumber || isInt || isInt64 || isFloat
	case ParamTypeString:
		_, rightType = value.(string)
	case ParamTypeArray:
		rightType = (typ.Kind() == reflect.Array) || (typ.Kind() == reflect.Slice)

		if rightType && param.ItemType != "" {
			reflectVal := reflect.ValueOf(value)
			for i := range reflectVal.Len() {
				if !a.rightType(&Param{Type: param.ItemType}, reflectVal.Index(i).Interface()) {
					rightType = false
					break
				}
			}
		}
	case ParamTypeObject:
		rightType = typ.Kind() == reflect.Map
	case ParamTypeNull:
		rightType = value == nil
	}

	return rightType
}

func (a Args) GetString(key string) *string {
	val, hasKey := a[key]
	if !hasKey {
		return nil
	}

	strVal, isStr := val.(string)
	if isStr {
		return &strVal
	}

	stringerVal, isStringer := val.(fmt.Stringer)
	if isStringer {
		val := stringerVal.String()
		return &val
	}

	return nil
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

	int64Val, isInt64 := val.(int64)
	if isInt64 {
		return &int64Val
	}

	intVal, isInt := val.(int)
	if isInt {
		convertedVal := int64(intVal)
		return &convertedVal
	}

	strVal, isStr := val.(string)
	if isStr {
		parsedVal, err := strconv.ParseInt(strVal, 10, 64)
		if err != nil {
			return nil
		}

		return &parsedVal
	}

	return nil
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
	if isFloat {
		return &floatVal
	}

	stringVal, isStr := val.(string)
	if isStr {
		parsedVal, err := strconv.ParseFloat(stringVal, 64)
		if err != nil {
			return nil
		}

		return &parsedVal
	}

	if intVal := a.GetInt64(key); intVal != nil {
		floatVal := float64(*intVal)
		return &floatVal
	}

	return nil
}

func (a Args) GetBool(key string) *bool {
	val, hasKey := a[key]
	if !hasKey {
		return nil
	}

	boolVal, isBool := val.(bool)
	if isBool {
		return &boolVal
	}

	strVal, isStr := val.(string)
	if isStr {
		parsed, err := strconv.ParseBool(strVal)
		if err != nil {
			return nil
		}

		return &parsed
	}

	return nil
}

func (a Args) GetStringSlice(key string) []string {
	val, hasKey := a[key]
	if !hasKey {
		return nil
	}

	sliceVal, isSlice := val.([]any)
	if !isSlice {
		stringSliceVal, isStringSlice := val.([]string)
		if isStringSlice {
			return stringSliceVal
		}

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
