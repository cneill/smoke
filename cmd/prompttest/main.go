package main

import (
	"fmt"

	"github.com/cneill/smoke/pkg/prompts"
)

func main() {
	printPrompts := []*prompts.Prompt{
		prompts.WorkSystemPrompt(),
		prompts.PlanningSystemPrompt(),
		prompts.ReviewSystemPrompt(),
		prompts.SummarizeSystemPrompt(),
	}

	for _, prompt := range printPrompts {
		fmt.Println("================================================")
		fmt.Println(prompt.Name)
		fmt.Println("================================================")
		md := prompt.Markdown()
		fmt.Println(md)

		json := prompt.JSON()
		fmt.Println(json)
	}
}
