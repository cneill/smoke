package llms

type Role string

const (
	RoleUnknown   Role = ""
	RoleSystem    Role = "system"
	RoleAssistant Role = "assistant"
	RoleUser      Role = "user"
	RoleTool      Role = "tool"
)
