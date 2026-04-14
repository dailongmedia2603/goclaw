// Package presets provides a registry of curated MCP server presets
// (Lark, GitHub, Slack…) that turn minimal user input into a ready-to-insert
// store.MCPServerData row.
//
// Each preset implements the Preset interface: exposes metadata for the UI
// catalog, validates form input, and produces the MCPServerData needed by
// the existing Manager/Pool machinery. Presets register themselves via
// init() → Register(). The HTTP layer looks presets up via Get(id).
package presets
