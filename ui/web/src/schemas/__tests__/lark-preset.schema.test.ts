import { describe, it, expect } from "vitest";
import {
  LARK_DEFAULT_VALUES,
  LARK_DOMAINS,
  LARK_DOMAIN_VALUES,
  LARK_TOOL_PRESETS,
  LARK_TOKEN_MODES,
  larkPresetSchema,
} from "../lark-preset.schema";

const validData = {
  display_name: "Lark Prod",
  app_id: "cli_abc123",
  app_secret: "mysecret",
  domain: "https://open.larksuite.com",
  token_mode: "tenant_access_token",
  tool_presets: ["preset.default"],
  timeout_sec: 90,
  enabled: true,
};

describe("larkPresetSchema", () => {
  it("accepts a fully valid payload", () => {
    const result = larkPresetSchema.safeParse(validData);
    expect(result.success).toBe(true);
  });

  it("rejects App ID without cli_ prefix", () => {
    const result = larkPresetSchema.safeParse({ ...validData, app_id: "abcdef" });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(JSON.stringify(result.error.issues)).toContain("cli_");
    }
  });

  it("rejects empty App ID", () => {
    const result = larkPresetSchema.safeParse({ ...validData, app_id: "" });
    expect(result.success).toBe(false);
  });

  it("rejects unknown domain", () => {
    const result = larkPresetSchema.safeParse({ ...validData, domain: "https://evil.com" });
    expect(result.success).toBe(false);
  });

  it("accepts Feishu domain", () => {
    const result = larkPresetSchema.safeParse({ ...validData, domain: "https://open.feishu.cn" });
    expect(result.success).toBe(true);
  });

  it("rejects empty tool_presets array", () => {
    const result = larkPresetSchema.safeParse({ ...validData, tool_presets: [] });
    expect(result.success).toBe(false);
  });

  it("rejects timeout below 10", () => {
    const result = larkPresetSchema.safeParse({ ...validData, timeout_sec: 5 });
    expect(result.success).toBe(false);
  });

  it("rejects timeout above 600", () => {
    const result = larkPresetSchema.safeParse({ ...validData, timeout_sec: 601 });
    expect(result.success).toBe(false);
  });

  it("rejects display_name longer than 80 chars", () => {
    const result = larkPresetSchema.safeParse({ ...validData, display_name: "x".repeat(81) });
    expect(result.success).toBe(false);
  });

  it("allows empty app_secret (edit mode form-level concern)", () => {
    const result = larkPresetSchema.safeParse({ ...validData, app_secret: "" });
    // Schema itself allows empty; the form enforces required-on-create manually
    expect(result.success).toBe(true);
  });

  it("rejects invalid token_mode", () => {
    const result = larkPresetSchema.safeParse({ ...validData, token_mode: "invalid_mode" });
    expect(result.success).toBe(false);
  });

  it("accepts both supported token modes", () => {
    for (const mode of LARK_TOKEN_MODES) {
      const result = larkPresetSchema.safeParse({ ...validData, token_mode: mode });
      expect(result.success).toBe(true);
    }
  });
});

describe("LARK_DEFAULT_VALUES", () => {
  it("matches the schema", () => {
    const result = larkPresetSchema.safeParse(LARK_DEFAULT_VALUES);
    // Note: app_id is empty in defaults, so it fails app_id.min validation —
    // that's intentional (user must fill it). Verify all other fields parse as types.
    expect(result.success).toBe(false);
    if (!result.success) {
      // Only app_id should fail (may have multiple issues: min + regex).
      const uniquePaths = Array.from(
        new Set(result.error.issues.map((i) => i.path.join("."))),
      );
      expect(uniquePaths).toEqual(["app_id"]);
    }
  });

  it("uses Lark International as default domain", () => {
    expect(LARK_DEFAULT_VALUES.domain).toBe("https://open.larksuite.com");
  });

  it("defaults to tenant_access_token mode", () => {
    expect(LARK_DEFAULT_VALUES.token_mode).toBe("tenant_access_token");
  });

  it("pre-selects the default tool preset", () => {
    expect(LARK_DEFAULT_VALUES.tool_presets).toContain("preset.default");
  });
});

describe("LARK_DOMAINS", () => {
  it("lists exactly two domains", () => {
    expect(LARK_DOMAINS).toHaveLength(2);
  });

  it("LARK_DOMAIN_VALUES matches the domain list", () => {
    expect(LARK_DOMAIN_VALUES).toEqual([
      "https://open.larksuite.com",
      "https://open.feishu.cn",
    ]);
  });
});

describe("LARK_TOOL_PRESETS", () => {
  it("includes all five curated presets", () => {
    const values = LARK_TOOL_PRESETS.map((p) => p.value);
    expect(values).toEqual([
      "preset.default",
      "preset.im.default",
      "preset.calendar.default",
      "preset.docs.default",
      "preset.contact.default",
    ]);
  });

  it("every preset has a labelKey in the presets.lark.tools namespace", () => {
    for (const p of LARK_TOOL_PRESETS) {
      expect(p.labelKey).toMatch(/^presets\.lark\.tools\./);
    }
  });
});
