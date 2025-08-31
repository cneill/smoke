package prompts

const Summarize = `Please summarize the conversation up to this point. Don't worry about details like "The user ` +
	`said...", just summarize the content of the current message history. Specifically focus on the outputs of tool ` +
	`calls and details that may be relevant to implementing any tasks described by the user. If no specific task is ` +
	`described, just summarize any information about the environment you're in. If there are tool calls present that ` +
	`would make earlier messages irrelevant, ignore the old content (for example, a "replace_lines" call deleting ` +
	`the content produced by a previous "read_file" call). Use Markdown headings to split the summary into logical ` +
	`groupings like "Package context and variable definitions", "Design decisions", and "Code conventions".`
