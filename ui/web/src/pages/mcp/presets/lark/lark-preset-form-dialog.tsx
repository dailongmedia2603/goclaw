import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Loader2, CheckCircle2, XCircle, ExternalLink, AlertCircle } from "lucide-react";

import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Alert, AlertDescription } from "@/components/ui/alert";

import type { MCPServerData } from "@/types/mcp";
import { useMCP } from "@/pages/mcp/hooks/use-mcp";
import { usePresetActions } from "@/pages/mcp/presets/use-presets";
import {
  LARK_DEFAULT_VALUES,
  LARK_DOMAINS,
  LARK_TOOL_PRESETS,
  larkPresetSchema,
  type LarkPresetFormData,
} from "@/schemas/lark-preset.schema";

interface LarkPresetFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  server?: MCPServerData | null;
}

interface TestResult {
  success: boolean;
  tool_count?: number;
  error?: string;
}

export function LarkPresetFormDialog({
  open,
  onOpenChange,
  server,
}: LarkPresetFormDialogProps) {
  const { t } = useTranslation("mcp");
  const isEdit = !!server;
  const { testConnection } = useMCP();
  const { createFromPreset, updateFromPreset } = usePresetActions();

  const defaults = useMemo<LarkPresetFormData>(() => {
    if (!server) return { ...LARK_DEFAULT_VALUES };
    const cfg = (server.settings?.preset_config ?? {}) as Partial<LarkPresetFormData>;
    return {
      display_name: (cfg.display_name as string) ?? server.display_name ?? "",
      app_id: (cfg.app_id as string) ?? "",
      app_secret: "",
      domain:
        ((cfg.domain as LarkPresetFormData["domain"]) ?? LARK_DEFAULT_VALUES.domain) as LarkPresetFormData["domain"],
      token_mode:
        ((cfg.token_mode as LarkPresetFormData["token_mode"]) ??
          LARK_DEFAULT_VALUES.token_mode) as LarkPresetFormData["token_mode"],
      tool_presets: (cfg.tool_presets as string[]) ?? ["preset.default"],
      timeout_sec: (cfg.timeout_sec as number) ?? server.timeout_sec ?? 90,
      enabled: server.enabled ?? true,
    };
  }, [server]);

  const form = useForm<LarkPresetFormData>({
    resolver: zodResolver(larkPresetSchema),
    mode: "onChange",
    defaultValues: defaults,
  });

  const { watch, setValue, register, handleSubmit, reset, formState } = form;
  const values = watch();

  useEffect(() => {
    if (open) {
      reset(defaults);
      setTestResult(null);
      setError("");
    }
  }, [open, defaults, reset]);

  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testResult, setTestResult] = useState<TestResult | null>(null);
  const [error, setError] = useState("");

  const buildTestPayload = (data: LarkPresetFormData) => ({
    transport: "stdio",
    command: "npx",
    args: [
      "-y",
      "@larksuiteoapi/lark-mcp",
      "mcp",
      "-a",
      data.app_id,
      "--domain",
      data.domain,
      "--token-mode",
      data.token_mode,
      "-t",
      data.tool_presets.join(","),
    ],
    env: {
      LARK_APP_ID: data.app_id,
      LARK_APP_SECRET: data.app_secret,
    },
  });

  const handleTest = async () => {
    setError("");
    const ok = await form.trigger(["app_id", "app_secret", "domain", "tool_presets"]);
    if (!ok) return;
    if (!values.app_secret) {
      form.setError("app_secret", { message: t("presets.lark.errors.secretRequiredForTest") });
      return;
    }
    setTesting(true);
    setTestResult(null);
    try {
      const result = await testConnection(buildTestPayload(values));
      setTestResult(result);
    } catch (err) {
      setTestResult({
        success: false,
        error: err instanceof Error ? err.message : t("form.errors.connectionFailed"),
      });
    } finally {
      setTesting(false);
    }
  };

  const onSubmit = handleSubmit(async (data) => {
    if (data.token_mode === "user_access_token") {
      setError(t("presets.lark.errors.userModeUnsupported"));
      return;
    }
    setSaving(true);
    setError("");
    try {
      const payload: Record<string, unknown> = {
        display_name: data.display_name?.trim() || undefined,
        app_id: data.app_id.trim(),
        domain: data.domain,
        token_mode: data.token_mode,
        tool_presets: data.tool_presets,
        timeout_sec: data.timeout_sec,
        enabled: data.enabled,
      };
      // Only include app_secret if user provided one (edit mode may leave blank).
      if (data.app_secret) payload.app_secret = data.app_secret;
      else if (!isEdit) {
        form.setError("app_secret", { message: t("presets.lark.errors.secretRequired") });
        setSaving(false);
        return;
      }

      if (isEdit && server) {
        await updateFromPreset("lark", server.id, payload);
      } else {
        await createFromPreset("lark", payload);
      }
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("form.errors.saveFailed", "Save failed"));
    } finally {
      setSaving(false);
    }
  });

  const toggleToolPreset = (value: string) => {
    const current = values.tool_presets;
    const next = current.includes(value)
      ? current.filter((v) => v !== value)
      : [...current, value];
    setValue("tool_presets", next, { shouldValidate: true, shouldDirty: true });
  };

  return (
    <Dialog open={open} onOpenChange={(v) => !saving && onOpenChange(v)}>
      <DialogContent
        className="max-h-[90vh] flex flex-col sm:max-w-xl"
        data-testid="lark-preset-form"
      >
        <DialogHeader>
          <DialogTitle>
            {isEdit ? t("presets.lark.editTitle") : t("presets.lark.createTitle")}
          </DialogTitle>
        </DialogHeader>

        <div className="grid gap-4 py-2 -mx-4 px-4 sm:-mx-6 sm:px-6 overflow-y-auto min-h-0">
          {/* Display Name */}
          <div className="grid gap-1.5">
            <Label htmlFor="lark-display-name">
              {t("presets.lark.fields.displayName")}{" "}
              <span className="text-xs text-muted-foreground">({t("form.optional")})</span>
            </Label>
            <Input
              id="lark-display-name"
              {...register("display_name")}
              placeholder={t("presets.lark.placeholders.displayName")}
              maxLength={80}
            />
          </div>

          {/* App ID */}
          <div className="grid gap-1.5">
            <Label htmlFor="lark-app-id">
              {t("presets.lark.fields.appId")} <span className="text-destructive">*</span>
            </Label>
            <Input
              id="lark-app-id"
              {...register("app_id")}
              placeholder="cli_a1b2c3d4e5f6..."
              autoComplete="off"
            />
            <p className="text-xs text-muted-foreground">
              {t("presets.lark.hints.findAppId")}{" "}
              <a
                href="https://open.larksuite.com/app"
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-0.5 text-primary hover:underline"
              >
                open.larksuite.com/app
                <ExternalLink className="h-3 w-3" />
              </a>
            </p>
            {formState.errors.app_id && (
              <p className="text-xs text-destructive">{formState.errors.app_id.message}</p>
            )}
          </div>

          {/* App Secret */}
          <div className="grid gap-1.5">
            <Label htmlFor="lark-app-secret">
              {t("presets.lark.fields.appSecret")}
              {!isEdit && <span className="text-destructive"> *</span>}
            </Label>
            <Input
              id="lark-app-secret"
              type="password"
              {...register("app_secret")}
              placeholder={isEdit ? t("presets.lark.hints.leaveEmptyToKeep") : ""}
              autoComplete="new-password"
            />
            {formState.errors.app_secret && (
              <p className="text-xs text-destructive">{formState.errors.app_secret.message}</p>
            )}
          </div>

          {/* Domain */}
          <div className="grid gap-1.5">
            <Label>
              {t("presets.lark.fields.domain")} <span className="text-destructive">*</span>
            </Label>
            <Select
              value={values.domain}
              onValueChange={(v) => setValue("domain", v as LarkPresetFormData["domain"], { shouldValidate: true })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {LARK_DOMAINS.map((d) => (
                  <SelectItem key={d.value} value={d.value}>
                    {t(d.labelKey)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Token Mode */}
          <div className="grid gap-1.5">
            <Label>
              {t("presets.lark.fields.tokenMode")} <span className="text-destructive">*</span>
            </Label>
            <RadioGroup
              value={values.token_mode}
              onValueChange={(v) =>
                setValue("token_mode", v as LarkPresetFormData["token_mode"], { shouldValidate: true })
              }
            >
              <div className="flex items-center gap-2">
                <RadioGroupItem value="tenant_access_token" id="lark-tm-tenant" />
                <Label htmlFor="lark-tm-tenant" className="cursor-pointer">
                  {t("presets.lark.tokenMode.tenant")}
                </Label>
              </div>
              <div className="flex items-center gap-2">
                <RadioGroupItem value="user_access_token" id="lark-tm-user" disabled />
                <Label htmlFor="lark-tm-user" className="cursor-not-allowed opacity-60">
                  {t("presets.lark.tokenMode.user")} ({t("common.comingSoon")})
                </Label>
              </div>
            </RadioGroup>
          </div>

          {values.token_mode === "user_access_token" && (
            <Alert>
              <AlertCircle className="h-4 w-4" />
              <AlertDescription>{t("presets.lark.userModeComingSoon")}</AlertDescription>
            </Alert>
          )}

          {/* Tool Presets */}
          <div className="grid gap-1.5">
            <Label>
              {t("presets.lark.fields.toolPresets")} <span className="text-destructive">*</span>
            </Label>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              {LARK_TOOL_PRESETS.map((p) => {
                const checked = values.tool_presets.includes(p.value);
                return (
                  <label
                    key={p.value}
                    className={
                      "flex items-start gap-2 rounded-md border p-2 cursor-pointer select-none transition " +
                      (checked ? "border-primary bg-accent/30" : "hover:bg-muted/40")
                    }
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={() => toggleToolPreset(p.value)}
                      className="mt-0.5"
                    />
                    <span className="text-sm">{t(p.labelKey)}</span>
                  </label>
                );
              })}
            </div>
            {formState.errors.tool_presets && (
              <p className="text-xs text-destructive">
                {formState.errors.tool_presets.message as string}
              </p>
            )}
          </div>

          {/* Timeout + Enabled */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <Label htmlFor="lark-timeout">{t("presets.lark.fields.timeoutSec")}</Label>
              <Input
                id="lark-timeout"
                type="number"
                min={10}
                max={600}
                {...register("timeout_sec", { valueAsNumber: true })}
              />
            </div>
            <div className="flex items-center gap-2 pt-6">
              <Switch
                id="lark-enabled"
                checked={values.enabled}
                onCheckedChange={(v) => setValue("enabled", v)}
              />
              <Label htmlFor="lark-enabled">{t("form.enabled")}</Label>
            </div>
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>

        <DialogFooter className="flex-col sm:flex-row gap-2">
          <div className="flex items-center gap-2 mr-auto">
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={handleTest}
              disabled={testing || saving}
            >
              {testing ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin mr-1" />
                  {t("form.testing")}
                </>
              ) : (
                t("form.testConnection")
              )}
            </Button>
            {testResult && (
              <span
                className={
                  "flex items-center gap-1 text-xs " +
                  (testResult.success
                    ? "text-emerald-600 dark:text-emerald-400"
                    : "text-destructive")
                }
              >
                {testResult.success ? (
                  <>
                    <CheckCircle2 className="h-3.5 w-3.5" />
                    {t("form.toolsFound", { count: testResult.tool_count ?? 0 })}
                  </>
                ) : (
                  <>
                    <XCircle className="h-3.5 w-3.5" />
                    {testResult.error}
                  </>
                )}
              </span>
            )}
          </div>
          <div className="flex gap-2">
            <Button
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={saving}
            >
              {t("form.cancel")}
            </Button>
            <Button onClick={onSubmit} disabled={saving}>
              {saving ? t("form.saving") : isEdit ? t("form.update") : t("form.create")}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
