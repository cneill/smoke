package main

import (
	"fmt"
)

func main() {
	// builder := NewBuilder("System: Go Assistant").
	// 	Add(SectionTask, P("You are a senior Go engineer. Provide accurate, efficient solutions.")).
	// 	Add(SectionRules, List(
	// 		Item("Be concise"),
	// 		Item("Prefer code examples", Item("Use fenced blocks with language hints")),
	// 	)).
	// 	Add(SectionFormatting, Code("markdown", "Always wrap code examples in ```go``` fences.")).
	// 	ApplyPreset(DefaultToneAndFormatting, Append)
	//
	// prompt := builder.Build()

	prompt := WorkSystemPrompt()

	md := MarkdownRenderer{}.Render(prompt)
	fmt.Println(md)

	json := JSONRenderer{}.RenderString(prompt)
	fmt.Println(json)
}
