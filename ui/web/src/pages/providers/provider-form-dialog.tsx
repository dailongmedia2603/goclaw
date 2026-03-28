import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
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
import type { ProviderData, ProviderInput } from "./hooks/use-providers";
import { slugify, isValidSlug } from "@/lib/slug";
import { PROVIDER_TYPES } from "@/constants/providers";
import { OAuthSection } from "./provider-oauth-section";
import { CLISection } from "./provider-cli-section";
import { ACPSection } from "./provider-acp-section";
import { useHttp } from "@/hooks/use-ws";
import { Loader2, CheckCircle2, XCircle } from "lucide-react";

interface ProviderFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (data: ProviderInput) => Promise<unknown>;
  existingProviders?: ProviderData[];
}

export function ProviderFormDialog({ open, onOpenChange, onSubmit, existingProviders = [] }: ProviderFormDialogProps) {
  const { t } = useTranslation("providers");
  const queryClient = useQueryClient();
  const http = useHttp();
  const [name, setName] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [providerType, setProviderType] = useState("openai_compat");
  const [apiBase, setApiBase] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [modelId, setModelId] = useState("");
  const [enabled, setEnabled] = useState(true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  // Test connection state
  const [testLoading, setTestLoading] = useState(false);
  const [testResult, setTestResult] = useState<{ valid: boolean; error?: string } | null>(null);

  // ACP fields
  const [acpBinary, setAcpBinary] = useState("");
  const [acpArgs, setAcpArgs] = useState("");
  const [acpIdleTTL, setAcpIdleTTL] = useState("");
  const [acpPermMode, setAcpPermMode] = useState("approve-all");
  const [acpWorkDir, setAcpWorkDir] = useState("");

  const hasClaudeCLI = existingProviders.some((p) => p.provider_type === "claude_cli");

  const isOAuth = providerType === "chatgpt_oauth";
  const isCLI = providerType === "claude_cli";
  const isACP = providerType === "acp";

  useEffect(() => {
    if (open) {
      setError("");
      setName("");
      setDisplayName("");
      setProviderType("openai_compat");
      setApiBase("");
      setApiKey("");
      setModelId("");
      setEnabled(true);
      setTestLoading(false);
      setTestResult(null);
      setAcpBinary("");
      setAcpArgs("");
      setAcpIdleTTL("");
      setAcpPermMode("approve-all");
      setAcpWorkDir("");
    }
  }, [open]);

  const handleSubmit = async () => {
    if (!name.trim() || !providerType) return;
    setLoading(true);
    try {
      const data: ProviderInput = {
        name: name.trim(),
        display_name: displayName.trim() || undefined,
        provider_type: providerType,
        api_base: apiBase.trim() || undefined,
        enabled,
      };

      if (isACP) {
        data.api_base = acpBinary.trim() || undefined;
        const settings: Record<string, unknown> = {};
        if (acpArgs.trim()) {
          settings.args = acpArgs.trim().split(/\s+/);
        }
        if (acpIdleTTL.trim()) settings.idle_ttl = acpIdleTTL.trim();
        if (acpPermMode) settings.perm_mode = acpPermMode;
        if (acpWorkDir.trim()) settings.work_dir = acpWorkDir.trim();
        if (Object.keys(settings).length > 0) {
          data.settings = settings;
        }
      }

      // Save default model ID in settings
      if (modelId.trim()) {
        data.settings = { ...data.settings, default_model: modelId.trim() };
      }

      if (apiKey && apiKey !== "***") {
        data.api_key = apiKey;
      }

      await onSubmit(data);
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("form.saving"));
    } finally {
      setLoading(false);
    }
  };

  // handleTestConnection: create provider → verify → if failed, delete it
  const handleTestConnection = async () => {
    if (!modelId.trim() || !name.trim() || !providerType) return;
    setTestLoading(true);
    setTestResult(null);
    let createdId: string | null = null;
    try {
      const data: ProviderInput = {
        name: name.trim(),
        display_name: displayName.trim() || undefined,
        provider_type: providerType,
        api_base: apiBase.trim() || undefined,
        enabled,
      };
      if (apiKey && apiKey !== "***") data.api_key = apiKey;
      if (modelId.trim()) data.settings = { default_model: modelId.trim() };

      const created = await http.post<{ id: string }>("/v1/providers", data);
      createdId = created.id;

      const result = await http.post<{ valid: boolean; error?: string }>(
        `/v1/providers/${created.id}/verify`,
        { model: modelId.trim() },
      );
      setTestResult(result);

      if (result.valid) {
        // Success — keep provider, close dialog
        queryClient.invalidateQueries({ queryKey: ["providers"] });
        onOpenChange(false);
      } else {
        // Failed — delete the temp provider
        try { await http.delete(`/v1/providers/${created.id}`); } catch {}
      }
    } catch (err) {
      setTestResult({ valid: false, error: err instanceof Error ? err.message : String(err) });
      // Clean up provider if created
      if (createdId) {
        try { await http.delete(`/v1/providers/${createdId}`); } catch {}
      }
    } finally {
      setTestLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[85vh] flex-col">
        <DialogHeader>
          <DialogTitle>{t("form.createTitle")}</DialogTitle>
          <DialogDescription>{t("form.configure")}</DialogDescription>
        </DialogHeader>
        <div className="-mx-4 min-h-0 overflow-y-auto px-4 py-4 sm:-mx-6 sm:px-6 space-y-4">
          <ProviderTypeSelect
            value={providerType}
            hasClaudeCLI={hasClaudeCLI}
            alreadyAddedLabel={t("form.alreadyAdded")}
            providerTypeLabel={t("form.providerType")}
            onChange={(v) => {
              setProviderType(v);
              const preset = PROVIDER_TYPES.find((pt) => pt.value === v);
              setApiBase(preset?.apiBase || "");
              if (v === "chatgpt_oauth") {
                setName("openai-codex");
                setDisplayName("ChatGPT (OAuth)");
              } else {
                if (name === "openai-codex") setName("");
                if (displayName === "ChatGPT (OAuth)") setDisplayName("");
              }
            }}
          />

          {isOAuth ? (
            <>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label>{t("form.nameFixed")}</Label>
                  <Input value="openai-codex" disabled className="text-base md:text-sm" />
                </div>
                <div className="space-y-2">
                  <Label>{t("form.displayName")}</Label>
                  <Input value="ChatGPT (OAuth)" disabled className="text-base md:text-sm" />
                </div>
              </div>
              <OAuthSection onSuccess={() => { queryClient.invalidateQueries({ queryKey: ["providers"] }); onOpenChange(false); }} />
            </>
          ) : (
            <>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="name">{t("form.name")}</Label>
                  <Input
                    id="name"
                    value={name}
                    onChange={(e) => setName(slugify(e.target.value))}
                    placeholder={t("form.namePlaceholder")}
                    className="text-base md:text-sm"
                  />
                  <p className="text-xs text-muted-foreground">{t("form.nameHint")}</p>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="displayName">{t("form.displayName")}</Label>
                  <Input
                    id="displayName"
                    value={displayName}
                    onChange={(e) => setDisplayName(e.target.value)}
                    placeholder={t("form.displayNamePlaceholder")}
                    className="text-base md:text-sm"
                  />
                </div>
              </div>

              {isCLI && <CLISection open={open} />}

              {isACP && (
                <ACPSection
                  binary={acpBinary}
                  onBinaryChange={setAcpBinary}
                  args={acpArgs}
                  onArgsChange={setAcpArgs}
                  idleTTL={acpIdleTTL}
                  onIdleTTLChange={setAcpIdleTTL}
                  permMode={acpPermMode}
                  onPermModeChange={setAcpPermMode}
                  workDir={acpWorkDir}
                  onWorkDirChange={setAcpWorkDir}
                />
              )}

              {!isCLI && !isACP && (
                <>
                  <div className="space-y-2">
                    <Label htmlFor="apiBase">{t("form.apiBase")}</Label>
                    <Input
                      id="apiBase"
                      value={apiBase}
                      onChange={(e) => setApiBase(e.target.value)}
                      placeholder={PROVIDER_TYPES.find((pt) => pt.value === providerType)?.placeholder || PROVIDER_TYPES.find((pt) => pt.value === providerType)?.apiBase || "https://api.example.com/v1"}
                      className="text-base md:text-sm"
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="apiKey">{t("form.apiKey")}</Label>
                    <Input
                      id="apiKey"
                      type="password"
                      value={apiKey}
                      onChange={(e) => setApiKey(e.target.value)}
                      placeholder={t("form.apiKeyPlaceholder")}
                      className="text-base md:text-sm"
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="modelId">{t("form.modelId")}</Label>
                    <Input
                      id="modelId"
                      value={modelId}
                      onChange={(e) => { setModelId(e.target.value); setTestResult(null); }}
                      placeholder={t("form.modelIdPlaceholder")}
                      className="text-base md:text-sm"
                    />
                    <p className="text-xs text-muted-foreground">{t("form.modelIdHint")}</p>
                  </div>
                </>
              )}

              <div className="flex items-center justify-between">
                <Label htmlFor="enabled">{t("form.enabled")}</Label>
                <Switch id="enabled" checked={enabled} onCheckedChange={setEnabled} />
              </div>
              {/* Test connection result */}
              {testResult && (
                <div className={`flex items-center gap-2 rounded-md border p-3 text-sm ${testResult.valid ? "border-green-500/30 bg-green-500/10 text-green-400" : "border-destructive/30 bg-destructive/10 text-destructive"}`}>
                  {testResult.valid ? <CheckCircle2 className="h-4 w-4 shrink-0" /> : <XCircle className="h-4 w-4 shrink-0" />}
                  <span>{testResult.valid ? t("form.testSuccess") : testResult.error || t("form.testFailed")}</span>
                </div>
              )}

              {error && (
                <p className="text-sm text-destructive">{error}</p>
              )}
            </>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={loading || testLoading}>
            {isOAuth ? t("form.close") : t("form.cancel")}
          </Button>
          {!isOAuth && (
            <div className="flex gap-2">
              {!isCLI && !isACP && modelId.trim() && (
                <Button
                  variant="outline"
                  onClick={handleTestConnection}
                  disabled={!name.trim() || !isValidSlug(name) || !providerType || !modelId.trim() || loading || testLoading}
                  className="gap-1"
                >
                  {testLoading && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                  {testLoading ? t("form.testing") : t("form.testConnection")}
                </Button>
              )}
              <Button
                onClick={handleSubmit}
                disabled={!name.trim() || !isValidSlug(name) || !providerType || loading || testLoading}
                className="gap-1"
              >
                {loading && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                {loading ? t("form.creating") : t("form.create")}
              </Button>
            </div>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function ProviderTypeSelect({ value, hasClaudeCLI, alreadyAddedLabel, providerTypeLabel, onChange }: {
  value: string;
  hasClaudeCLI: boolean;
  alreadyAddedLabel: string;
  providerTypeLabel: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="space-y-2">
      <Label>{providerTypeLabel}</Label>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {PROVIDER_TYPES.map((pt) => (
            <SelectItem
              key={pt.value}
              value={pt.value}
              disabled={pt.value === "claude_cli" && hasClaudeCLI}
            >
              {pt.label}
              {pt.value === "claude_cli" && hasClaudeCLI && (
                <span className="ml-1 text-xs opacity-60">{alreadyAddedLabel}</span>
              )}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
