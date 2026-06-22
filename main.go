// Command aks-helper manages connections to Azure Kubernetes Service clusters.
package main

import (
	"embed"

	"github.com/cesbron-dev/aks-helper/cmd"
)

// skillFS embeds the coding-agent skill so `aks-helper skill install` works from
// just the binary, with no checkout of the repository.
//
//go:embed all:.claude/skills/aks-access
var skillFS embed.FS

func main() {
	cmd.SkillFS = skillFS
	cmd.Execute()
}
