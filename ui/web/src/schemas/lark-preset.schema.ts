import { z } from "zod";

export const LARK_DOMAINS = [
  { value: "https://open.larksuite.com", labelKey: "presets.lark.domains.international" },
  { value: "https://open.feishu.cn", labelKey: "presets.lark.domains.feishu" },
] as const;

export const LARK_DOMAIN_VALUES = LARK_DOMAINS.map((d) => d.value) as [string, ...string[]];

export const LARK_TOOL_PRESETS = [
  { value: "preset.default", labelKey: "presets.lark.tools.default" },
  { value: "preset.im.default", labelKey: "presets.lark.tools.im" },
  { value: "preset.calendar.default", labelKey: "presets.lark.tools.calendar" },
  { value: "preset.docs.default", labelKey: "presets.lark.tools.docs" },
  { value: "preset.contact.default", labelKey: "presets.lark.tools.contact" },
] as const;

export const LARK_TOKEN_MODES = ["tenant_access_token", "user_access_token"] as const;

/**
 * Unified schema — app_secret is always a string (possibly empty). The form
 * component enforces the "secret required on create" rule outside the schema,
 * so create vs edit can share a single type without fighting react-hook-form's
 * inferred input/output differences.
 */
export const larkPresetSchema = z.object({
  display_name: z.string().trim().max(80),
  app_id: z
    .string()
    .trim()
    .min(5, "App ID is required")
    .regex(/^cli_[A-Za-z0-9]+$/, "App ID must start with cli_"),
  app_secret: z.string().trim().max(200),
  domain: z.enum(LARK_DOMAIN_VALUES),
  token_mode: z.enum(LARK_TOKEN_MODES),
  tool_presets: z.array(z.string()).min(1, "Select at least one tool preset"),
  timeout_sec: z.number().int().min(10).max(600),
  enabled: z.boolean(),
});

export type LarkPresetFormData = z.infer<typeof larkPresetSchema>;

export const LARK_DEFAULT_VALUES: LarkPresetFormData = {
  display_name: "",
  app_id: "",
  app_secret: "",
  domain: "https://open.larksuite.com",
  token_mode: "tenant_access_token",
  tool_presets: ["preset.default"],
  timeout_sec: 90,
  enabled: true,
};
