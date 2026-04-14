import { describe, it, expect } from "vitest";
import enMcp from "@/i18n/locales/en/mcp.json";
import viMcp from "@/i18n/locales/vi/mcp.json";
import zhMcp from "@/i18n/locales/zh/mcp.json";

/**
 * Lock down the Lark preset i18n tree across all three supported locales.
 * Missing a key in one locale breaks the UI at runtime — this test catches
 * regressions at build time.
 */

const REQUIRED_KEYS: string[] = [
  "addFromPreset",
  "addFromPresetHint",
  "addCustom",
  "addCustomHint",
  "common.comingSoon",
  "presets.catalog.title",
  "presets.catalog.description",
  "presets.catalog.select",
  "presets.custom.title",
  "presets.custom.description",
  "presets.lark.createTitle",
  "presets.lark.editTitle",
  "presets.lark.fields.displayName",
  "presets.lark.fields.appId",
  "presets.lark.fields.appSecret",
  "presets.lark.fields.domain",
  "presets.lark.fields.tokenMode",
  "presets.lark.fields.toolPresets",
  "presets.lark.fields.timeoutSec",
  "presets.lark.placeholders.displayName",
  "presets.lark.hints.findAppId",
  "presets.lark.hints.leaveEmptyToKeep",
  "presets.lark.domains.international",
  "presets.lark.domains.feishu",
  "presets.lark.tokenMode.tenant",
  "presets.lark.tokenMode.user",
  "presets.lark.tools.default",
  "presets.lark.tools.im",
  "presets.lark.tools.calendar",
  "presets.lark.tools.docs",
  "presets.lark.tools.contact",
  "presets.lark.errors.secretRequired",
  "presets.lark.errors.secretRequiredForTest",
  "presets.lark.errors.userModeUnsupported",
  "presets.lark.userModeComingSoon",
  "form.optional",
];

function getPath(obj: unknown, path: string): unknown {
  return path.split(".").reduce<unknown>((acc, k) => {
    if (acc && typeof acc === "object" && k in (acc as Record<string, unknown>)) {
      return (acc as Record<string, unknown>)[k];
    }
    return undefined;
  }, obj);
}

describe("Lark preset i18n", () => {
  for (const [name, catalog] of Object.entries({ en: enMcp, vi: viMcp, zh: zhMcp })) {
    describe(`locale ${name}`, () => {
      for (const key of REQUIRED_KEYS) {
        it(`has key ${key}`, () => {
          const value = getPath(catalog, key);
          expect(value, `${name}.${key} missing`).toBeTypeOf("string");
          expect(value).not.toBe("");
        });
      }
    });
  }
});
