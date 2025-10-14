package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// Args is used as a generic container for accepting tool call information from the LLM. Its GetX methods allow for
// retrieving these values by name based on the type they are expected to have. Args of the wrong type, or that were
// not provided at all, return nil.
type Args map[string]any

func ParseArgs(params Params, input []byte) (Args, error) {
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

	if err := result.checkValues(params); err != nil {
		return nil, err
	}

	return result, nil
}

func (a Args) String() string {
	argBytes, err := json.Marshal(a)
	if err != nil {
		// TODO: don't panic here? ignore error entirely?
		panic(fmt.Errorf("failed to marshal Args to bytes: %w", err))
	}

	return string(argBytes)
}

// GetString checks whether the argument matching 'key' is either a string or a [fmt.Stringer] and returns the string
// value if applicable, or nil if it is undefined or of another tpye.
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

// GetInt checks whether the argument matching 'key' is either a [json.Number], an int64, an int, or a string, and
// handles it appropriately to return an int. If the int64 value can't be safely converted to an int, or if the type is
// not a reasonable one for an int, it returns nil.
func (a Args) GetInt(key string) *int {
	int64Val := a.GetInt64(key)
	if int64Val == nil {
		return nil
	} else if *int64Val < math.MinInt || *int64Val > math.MaxInt {
		return nil
	}

	intVal := int(*int64Val)

	return &intVal
}

// GetInt64 checks whether the argument matching 'key' is either a [json.Number], an int64, an int, or a string, and
// handles it appropriately to return an int64. If none of the above, it returns nil.
func (a Args) GetInt64(key string) *int64 {
	val, hasKey := a[key]
	if !hasKey {
		return nil
	}

	var (
		int64Val int64
		err      error
	)

	switch val := val.(type) {
	case json.Number:
		int64Val, err = val.Int64()
		if err != nil {
			return nil
		}
	case int:
		int64Val = int64(val)
	case string:
		int64Val, err = strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil
		}
	case int64:
		int64Val = val
	default:
		return nil
	}

	return &int64Val
}

// GetFloat64 checks whether the argument matching 'key' is a [json.Number], a float64, or a string, and if not, it
// tries to treat it as an int64 by calling [Args.GetInt64]. If none of the above, it returns nil.
func (a Args) GetFloat64(key string) *float64 {
	val, hasKey := a[key]
	if !hasKey {
		return nil
	}

	var (
		float64Val float64
		err        error
	)

	switch val := val.(type) {
	case json.Number:
		float64Val, err = val.Float64()
		if err != nil {
			return nil
		}
	case string:
		float64Val, err = strconv.ParseFloat(val, 64)
		if err != nil {
			return nil
		}
	case float64:
		float64Val = val
	case int, int64:
		if intVal := a.GetInt64(key); intVal != nil {
			float64Val = float64(*intVal)
		}
	default:
		return nil
	}

	return &float64Val
}

// GetBool checks whether the argument matching 'key' is a bool or a string, handling the conversion if necessary, or
// returns nil.
func (a Args) GetBool(key string) *bool {
	val, hasKey := a[key]
	if !hasKey {
		return nil
	}

	var (
		boolVal bool
		err     error
	)

	switch val := val.(type) {
	case string:
		boolVal, err = strconv.ParseBool(val)
		if err != nil {
			return nil
		}
	case bool:
		boolVal = val
	default:
		return nil
	}

	return &boolVal
}

func (a Args) GetArgsObject(key string) Args {
	val, hasKey := a[key]
	if !hasKey {
		return nil
	}

	objVal, isObj := val.(map[string]any)
	if !isObj {
		return nil
	}

	return objVal
}

// GetStringSlice first checks that we have a slice of []any, then converts this into a []string slice. If any elements
// are not strings, it returns nil.
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

	for itemIdx, rawVal := range sliceVal {
		strVal, ok := rawVal.(string)
		// be brutal - no non-strings allowed
		if !ok {
			return nil
		}

		stringSlice[itemIdx] = strVal
	}

	return stringSlice
}

func (a Args) GetArgsObjectSlice(key string) []Args {
	val, hasKey := a[key]
	if !hasKey {
		return nil
	}

	sliceVal, isSlice := val.([]any)
	if !isSlice {
		return nil
	}

	objectSlice := make([]Args, len(sliceVal))

	for itemIdx, rawVal := range sliceVal {
		objVal, ok := rawVal.(map[string]any)
		if !ok {
			return nil
		}

		objectSlice[itemIdx] = objVal
	}

	return objectSlice
}

// LogValue helps with rendering [Args] in a slog message.
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

// checkTypes walks all the keys in the slice and ensures that they match the types expected by the corresponding
// [Param], returning an error if any types are mismatched.
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

// rightType checks an individual arg 'value' against the expected [Param] type. It also calls itself to check
// [Param.ItemType] if the [Param.Type] is [ParamTypeArray].
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

func (a Args) checkValues(params Params) error {
	for key, value := range a {
		param := params.ByKey(key)
		switch param.Type { //nolint:exhaustive
		case ParamTypeString:
			if err := a.checkStringValue(param, value); err != nil {
				return fmt.Errorf("invalid string value for param %q: %w", key, err)
			}
		case ParamTypeObject:
			if err := a.checkObjectValue(param, value); err != nil {
				return fmt.Errorf("invalid object value for param %q: %w", key, err)
			}
		case ParamTypeArray:
			if err := a.checkArrayValues(param, value); err != nil {
				return fmt.Errorf("invalid value for param %q: %w", key, err)
			}
		}
	}

	return nil
}

func (a Args) checkStringValue(param *Param, argValue any) error {
	if len(param.EnumStringValues) == 0 {
		return nil
	}

	strVal, ok := argValue.(string)
	if !ok {
		return fmt.Errorf("%w: got non-string argument value", ErrWrongTypeKeys)
	}

	if !slices.Contains(param.EnumStringValues, strVal) {
		return fmt.Errorf("%w: not one of the expected enum values %s", ErrUnexpectedValue, strings.Join(param.EnumStringValues, ", "))
	}

	return nil
}

func (a Args) checkObjectValue(param *Param, argValue any) error {
	if param.NestedParams == nil {
		return nil
	}

	marshalled, err := json.Marshal(argValue)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal object value to JSON: %w", ErrInvalidJSON, err)
	}

	if _, err := ParseArgs(param.NestedParams, marshalled); err != nil {
		return fmt.Errorf("%w: failed to parse nested object: %w", ErrUnexpectedValue, err)
	}

	return nil
}

// TODO: complete
func (a Args) checkArrayValues(_ *Param, _ any) error {
	return nil
}
