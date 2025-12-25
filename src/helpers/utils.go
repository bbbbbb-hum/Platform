package helpers

import "strings"

// 从路径中提取服务ID
func ExtractServerID(path string) string {
	// /mcp-server/{server_id}/...
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "mcp-server" {
		return parts[1]
	}
	return ""
}
