// Package internal 包含内部逻辑定义
package internal

import "fmt"

var (
	// CommitID git commit hash
	CommitID = ""

	// BuildTime when to build
	BuildTime = ""

	// Version what version
	Version = ""
)

// VersionInfo print current cmd version
func VersionInfo() string {
	return fmt.Sprintf("\nCurrent version: %s\nCommit hash: %s\nBuild time: %s\n\n", Version, CommitID, BuildTime)
}
