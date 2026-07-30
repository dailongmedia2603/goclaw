import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import {
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Loader2, ExternalLink } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";

interface NhanhSettings {
  app_id: string;
  business_id: string;
  access_token: string;
  auto_kg_ingest: boolean;
}

const defaultSettings: NhanhSettings = {
  app_id: "",
  business_id: "",
  access_token: "",
  auto_kg_ingest: true,
};

interface Props {
  initialSettings: Record<string, unknown>;
  onSave: (settings: Record<string, unknown>) => Promise<void>;
  onCancel: () => void;
}

export function NhanhSettingsForm({ initialSettings, onSave, onCancel }: Props) {
  const { t } = useTranslation("tools");
  const [settings, setSettings] = useState<NhanhSettings>(defaultSettings);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setSettings({
      ...defaultSettings,
      ...initialSettings,
      auto_kg_ingest: initialSettings.auto_kg_ingest !== false,
    });
  }, [initialSettings]);

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave(settings as unknown as Record<string, unknown>);
    } catch {
      // toast shown by hook
    } finally {
      setSaving(false);
    }
  };

  const isValid = settings.app_id.trim() !== "" &&
    settings.business_id.trim() !== "" &&
    settings.access_token.trim() !== "";

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t("builtin.nhanhSettings.title")}</DialogTitle>
        <DialogDescription>
          {t("builtin.nhanhSettings.description")}
        </DialogDescription>
      </DialogHeader>

      <div className="space-y-4 py-2">
        <div className="grid gap-1.5">
          <Label htmlFor="nhanh-app-id" className="text-sm">
            {t("builtin.nhanhSettings.appId")}
          </Label>
          <Input
            id="nhanh-app-id"
            type="text"
            value={settings.app_id}
            onChange={(e) => setSettings((s) => ({ ...s, app_id: e.target.value }))}
            placeholder="e.g. 12345"
            className="text-base md:text-sm"
          />
          <p className="text-xs text-muted-foreground">
            {t("builtin.nhanhSettings.appIdHint")}
          </p>
        </div>

        <div className="grid gap-1.5">
          <Label htmlFor="nhanh-business-id" className="text-sm">
            {t("builtin.nhanhSettings.businessId")}
          </Label>
          <Input
            id="nhanh-business-id"
            type="text"
            value={settings.business_id}
            onChange={(e) => setSettings((s) => ({ ...s, business_id: e.target.value }))}
            placeholder="e.g. 56143"
            className="text-base md:text-sm"
          />
          <p className="text-xs text-muted-foreground">
            {t("builtin.nhanhSettings.businessIdHint")}
          </p>
        </div>

        <div className="grid gap-1.5">
          <Label htmlFor="nhanh-access-token" className="text-sm">
            {t("builtin.nhanhSettings.accessToken")}
          </Label>
          <Input
            id="nhanh-access-token"
            type="password"
            value={settings.access_token}
            onChange={(e) => setSettings((s) => ({ ...s, access_token: e.target.value }))}
            placeholder="••••••••"
            className="text-base md:text-sm"
          />
          <p className="text-xs text-muted-foreground">
            {t("builtin.nhanhSettings.accessTokenHint")}
          </p>
        </div>

        <div className="flex items-center justify-between rounded-md border p-3">
          <div>
            <Label htmlFor="nhanh-auto-kg" className="text-sm font-medium">
              {t("builtin.nhanhSettings.autoKgIngest")}
            </Label>
            <p className="text-xs text-muted-foreground mt-0.5">
              {t("builtin.nhanhSettings.autoKgIngestHint")}
            </p>
          </div>
          <Switch
            id="nhanh-auto-kg"
            checked={settings.auto_kg_ingest}
            onCheckedChange={(v) => setSettings((s) => ({ ...s, auto_kg_ingest: v }))}
          />
        </div>

        <a
          href="https://open.nhanh.vn"
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
        >
          {t("builtin.nhanhSettings.helpLink")}
          <ExternalLink className="h-3 w-3" />
        </a>
      </div>

      <DialogFooter>
        <Button variant="outline" onClick={onCancel}>
          {t("builtin.nhanhSettings.cancel")}
        </Button>
        <Button onClick={handleSave} disabled={saving || !isValid}>
          {saving && <Loader2 className="h-4 w-4 animate-spin" />}
          {saving ? t("builtin.nhanhSettings.saving") : t("builtin.nhanhSettings.save")}
        </Button>
      </DialogFooter>
    </>
  );
}
