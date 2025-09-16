package prompts

import (
	"encoding/json"
	"fmt"
)

func ReviewJSON() string {
	reviewJSON := map[string]any{
		"task": `Review the user's code and note any areas that match one of the "red flags" described here, and ` +
			`make suggestions for how the user could improve it. Note the name of the red flag that was violated and ` +
			`why you think the code is affected by that red flag.`,
		"red_flags": map[string]string{
			// p. 25
			"shallow_modules": "A **shallow module** is one whose interface is complicated relative to the " +
				"functionality it provides. Shallow modules don't help much in the battle against complexity, " +
				"because the benefit they provide (not having to learn about how they work internally) is negated by " +
				"the cost of learning and using their interfaces. Small modules tend to be shallow.",
			// p. 31
			"information_leakage": "Information leakage occurs when the same knowledge is used in multiple places, " +
				"such as two different classes that both understand the format of a particular type of file.",
			// p. 32
			// "temporal_decomposition": "In temporal decomposition, execution order is reflected in the code " +
			// 	"structure: operations that happen at different times are in different methods or classes. If the " +
			// 	"same knowledge is used at different points in execution, it gets encoded in multiple places, " +
			// 	"resulting in information leakage.",
			// p. 36
			"overexposure": "If the API for a commonly used feature forces users to learn about other features that " +
				"are rarely used, this increases the cognitive load on users who don't need the rarely used features.",
			// p. 52
			"pass_through_method": "A pass-through method is one that does nothing except pass its arguments to " +
				"another method, usually with the same API as the pass-through method. This typically indicates that " +
				"there is not a clean division of responsibility between the classes.",
			// p.68
			"repetition": "If the same piece of code (or code that is almost the same) appears over and over again, " +
				"that's a red flag that you haven't found the right abstractions.",
			// p. 71
			"special_and_general_mixture": "This red flag occurs when a general-purpose mechanism also contains code " +
				"specialized for a particular use of that mechanism. This makes the mechanism more complicated and " +
				"creates information leakage between the mechanism and the particular use case: future modifications " +
				"to the use case are likely to require changes to the underlying mechanism as well.",
			// p. 75
			"conjoined_methods": "It should be possible to understand each method independently. If you can't " +
				"understand the implementation of one method without also understanding the implementation of " +
				"another, that's a red flag. This red flag can occur in other contexts as well: if two pieces of " +
				"code are physically separated, but each can only be understood by looking at the other, that is a " +
				"red flag.",
		},
		"tips": []string{
			"Think hard about how a user's code might be affected by one of these red flags.",
			`For the "repetition" red flag, focus on cases with >3 repetitions of almost identical code.`,
		},
	}

	bytes, err := json.Marshal(reviewJSON)
	if err != nil {
		panic(fmt.Errorf("failed to marshal reviewJSON: %w", err))
	}

	return string(bytes)
}
