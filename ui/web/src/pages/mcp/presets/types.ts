/**
 * Preset metadata returned by GET /v1/mcp/presets.
 * Mirror of backend presets.PresetMetadata in internal/mcp/presets.
 */
export interface PresetMetadata {
  id: string;
  display_name: string;
  description: string;
  icon: string;       // data: URI (SVG) or http URL
  doc_url?: string;
  schema?: Record<string, unknown>;
  defaults?: Record<string, unknown>;
}
