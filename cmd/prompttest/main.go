package main

import (
	"fmt"

	"github.com/cneill/smoke/pkg/prompts"
)

func main() {
	prompt := prompts.WorkSystemPrompt()

	md := prompt.Markdown()
	fmt.Println(md)

	json := prompt.JSON()
	fmt.Println(json)
}
