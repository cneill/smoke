package prompts

import (
	"encoding/json"
	"fmt"
)

func ReviewJSON() string {
	reviewJSON := map[string]any{
		"task": `Review the user's code and note any areas that match one of the "red flags" described here, and ` +
			`make suggestions for how the user could improve it. Note the name of the red flag that was violated and ` +
			`why you think the code is affected by that red flag. If the user asks for another review, re-read all` +
			`the files for the latest changes with "read_file" before providing your assessment.`,
		// Full list on p. 183
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
			// p. 104
			"comment_repeats_code": "If the information in a comment is already obvious from the code next to the " +
				"comment, then the comment isn't helpful. One example of this is when the comment uses the same " +
				"words that make up the name of the thing it is describing.",
			// p. 114
			"implementation_documentation_contaminates_interface": "This red flag occurs when interface " +
				"documentation, such as that for a method, describes implementation details that aren't needed in " +
				"order to use the thing being documented.",
			// p. 123
			"vague_name": "If a variable or method name is broad enough to refer to many different things, then it " +
				"doesn't convey much information to the developer and the underlying entity is more likely to be " +
				"misused.",
			// p. 125
			// "hard_to_pick_name": "If it's hard to find a simple name for a variable or method that creates a clear " +
			// 	"image of the underlying object, that's a hint that the underlying object may not have a clean design.",
			// p. 133
			// "hard_to_describe": "The comment that describes a method or a variable should be simple and yet " +
			// 	"complete. If you find it difficult to write such a comment, that's an indicator that there may be a " +
			// 	"problem with the design of the thing you are describing.",
			// p. 150
			"nonobvious_code": "If the meaning and behavior of code cannot be understood with a quick reading, it is " +
				"a red flag. Often this means that there is important information that is not immediately clear to " +
				"someone reading the code.",
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
