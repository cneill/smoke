package tools

import (
	"encoding/json"
	"fmt"
)

func ExampleJSONParams(args ...any) (string, error) {
	argc := len(args)

	if argc%2 != 0 {
		return "", fmt.Errorf("example args must have key + val pairs, got non-even number of args")
	}

	resultsMap := map[string]any{}

	for argNum := 0; argNum < argc-1; argNum += 2 {
		argName, ok := args[argNum].(string)
		if !ok {
			return "", fmt.Errorf("argument %d was not a string (%T): %+v", argNum, args[argNum], args[argNum])
		}

		argVal := args[argNum+1]

		if _, ok := resultsMap[argName]; ok {
			return "", fmt.Errorf("got same argument (%s) more than once", argName)
		}

		resultsMap[argName] = argVal
	}

	exampleJSON, err := json.Marshal(resultsMap)
	if err != nil {
		return "", fmt.Errorf("%w: error generating example JSON: %w", ErrInvalidJSON, err)
	}

	return string(exampleJSON), nil
}
