import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Globe,
  RefreshCw,
  Plus,
  Monitor,
  Wifi,
  WifiOff,
  Lock,
  Unlock,
  ExternalLink,
  Pencil,
  Trash2,
  X,
} from "lucide-react";
import { PageHeader } from "@/components/shared/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import {
  useBrowserProfiles,
  type BrowserProfile,
  type ProfileConfig,
} from "./hooks/use-browser-profiles";

export function BrowserPage() {
  const { t } = useTranslation("browser");
  const { profiles, loading, saving, refresh, saveProfiles } =
    useBrowserProfiles();
  const [refreshing, setRefreshing] = useState(false);
  const [editingProfile, setEditingProfile] = useState<ProfileFormData | null>(
    null,
  );
  const [dialogOpen, setDialogOpen] = useState(false);
  const [vncProfile, setVncProfile] = useState<BrowserProfile | null>(null);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const handleRefresh = async () => {
    setRefreshing(true);
    await refresh();
    setTimeout(() => setRefreshing(false), 500);
  };

  const handleAdd = () => {
    setEditingProfile({
      name: "",
      remote_url: "",
      headless: false,
      shared: false,
      domains: "",
      vnc_url: "",
      action_timeout_ms: 30000,
      idle_timeout_ms: 600000,
      max_pages: 5,
      isNew: true,
    });
    setDialogOpen(true);
  };

  const handleEdit = (p: BrowserProfile) => {
    setEditingProfile({
      name: p.name,
      remote_url: "",
      headless: false,
      shared: p.shared,
      domains: p.domains?.join(", ") ?? "",
      vnc_url: p.vnc_url ?? "",
      action_timeout_ms: 30000,
      idle_timeout_ms: 600000,
      max_pages: 5,
      isNew: false,
    });
    setDialogOpen(true);
  };

  const handleSave = async (form: ProfileFormData) => {
    const profilesMap: Record<string, ProfileConfig> = {};

    // Keep existing profiles
    for (const p of profiles) {
      if (p.name !== form.name || form.isNew) {
        profilesMap[p.name] = {
          shared: p.shared,
          domains: p.domains,
          vnc_url: p.vnc_url || undefined,
        };
      }
    }

    // Add/update the edited profile
    const domains = form.domains
      .split(",")
      .map((d) => d.trim())
      .filter(Boolean);
    profilesMap[form.name] = {
      remote_url: form.remote_url || undefined,
      headless: !form.remote_url ? form.headless : undefined,
      shared: form.shared,
      domains: domains.length > 0 ? domains : undefined,
      vnc_url: form.vnc_url || undefined,
      action_timeout_ms: form.action_timeout_ms || undefined,
      idle_timeout_ms: form.idle_timeout_ms || undefined,
      max_pages: form.max_pages || undefined,
    };

    await saveProfiles(profilesMap);
    setDialogOpen(false);
  };

  const handleDelete = async (name: string) => {
    const profilesMap: Record<string, ProfileConfig> = {};
    for (const p of profiles) {
      if (p.name !== name) {
        profilesMap[p.name] = {
          shared: p.shared,
          domains: p.domains,
          vnc_url: p.vnc_url || undefined,
        };
      }
    }
    await saveProfiles(profilesMap);
  };

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title={t("title")}
        description={t("description")}
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={handleRefresh}>
              <RefreshCw
                className={cn("mr-2 h-4 w-4", refreshing && "animate-spin")}
              />
              {t("refresh")}
            </Button>
            <Button size="sm" onClick={handleAdd}>
              <Plus className="mr-2 h-4 w-4" />
              {t("addProfile")}
            </Button>
          </div>
        }
      />

      {/* Profiles list */}
      {loading && !profiles.length ? (
        <div className="space-y-4">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-24 w-full" />
        </div>
      ) : profiles.length === 0 ? (
        <Card className="flex flex-col items-center justify-center p-12 text-center">
          <Globe className="mb-4 h-12 w-12 text-muted-foreground/50" />
          <p className="text-lg font-medium">{t("empty.title")}</p>
          <p className="mt-1 text-sm text-muted-foreground">
            {t("empty.description")}
          </p>
          <Button className="mt-4" onClick={handleAdd}>
            <Plus className="mr-2 h-4 w-4" />
            {t("addProfile")}
          </Button>
        </Card>
      ) : (
        <div className="grid gap-4">
          {profiles.map((p) => (
            <ProfileCard
              key={p.name}
              profile={p}
              onEdit={() => handleEdit(p)}
              onDelete={() => handleDelete(p.name)}
              onVnc={() => setVncProfile(p)}
              t={t}
            />
          ))}
        </div>
      )}

      {/* VNC Viewer */}
      {vncProfile && vncProfile.vnc_url && (
        <Card className="overflow-hidden">
          <div className="flex items-center justify-between border-b px-4 py-3">
            <div className="flex items-center gap-2">
              <Monitor className="h-4 w-4" />
              <span className="font-medium">
                {t("vnc.title", { name: vncProfile.name })}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() =>
                  window.open(vncProfile.vnc_url, "_blank", "noopener")
                }
              >
                <ExternalLink className="mr-2 h-3 w-3" />
                {t("vnc.openNew")}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setVncProfile(null)}
              >
                <X className="h-4 w-4" />
              </Button>
            </div>
          </div>
          <div className="relative h-[600px] w-full bg-black">
            <iframe
              src={vncProfile.vnc_url}
              className="h-full w-full border-0"
              title={`VNC - ${vncProfile.name}`}
              sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
            />
          </div>
        </Card>
      )}

      {/* Profile Edit Dialog */}
      <ProfileDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        profile={editingProfile}
        saving={saving}
        onSave={handleSave}
        t={t}
      />
    </div>
  );
}

/* ─── Sub-components ─── */

interface ProfileFormData {
  name: string;
  remote_url: string;
  headless: boolean;
  shared: boolean;
  domains: string;
  vnc_url: string;
  action_timeout_ms: number;
  idle_timeout_ms: number;
  max_pages: number;
  isNew: boolean;
}

function ProfileCard({
  profile: p,
  onEdit,
  onDelete,
  onVnc,
  t,
}: {
  profile: BrowserProfile;
  onEdit: () => void;
  onDelete: () => void;
  onVnc: () => void;
  t: (key: string, opts?: Record<string, unknown>) => string;
}) {
  return (
    <Card className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex items-start gap-3">
        <div
          className={cn(
            "mt-0.5 flex h-10 w-10 items-center justify-center rounded-lg",
            p.running
              ? "bg-emerald-500/10 text-emerald-500"
              : "bg-muted text-muted-foreground",
          )}
        >
          <Globe className="h-5 w-5" />
        </div>
        <div>
          <div className="flex items-center gap-2">
            <span className="font-semibold">{p.name}</span>
            {p.running ? (
              <Badge
                variant="outline"
                className="border-emerald-500/30 text-emerald-500"
              >
                <Wifi className="mr-1 h-3 w-3" />
                {t("status.running", { tabs: p.tabs })}
              </Badge>
            ) : (
              <Badge variant="outline" className="text-muted-foreground">
                <WifiOff className="mr-1 h-3 w-3" />
                {t("status.stopped")}
              </Badge>
            )}
            {p.shared ? (
              <Badge variant="secondary">
                <Unlock className="mr-1 h-3 w-3" />
                {t("mode.shared")}
              </Badge>
            ) : (
              <Badge variant="secondary">
                <Lock className="mr-1 h-3 w-3" />
                {t("mode.isolated")}
              </Badge>
            )}
          </div>
          {p.domains && p.domains.length > 0 && (
            <p className="mt-1 text-xs text-muted-foreground">
              {t("domains")}: {p.domains.join(", ")}
            </p>
          )}
        </div>
      </div>
      <div className="flex items-center gap-2">
        {p.vnc_url && (
          <Button variant="outline" size="sm" onClick={onVnc}>
            <Monitor className="mr-2 h-3 w-3" />
            {t("vnc.login")}
          </Button>
        )}
        <Button variant="ghost" size="icon" onClick={onEdit}>
          <Pencil className="h-4 w-4" />
        </Button>
        {p.name !== "default" && (
          <Button variant="ghost" size="icon" onClick={onDelete}>
            <Trash2 className="h-4 w-4 text-destructive" />
          </Button>
        )}
      </div>
    </Card>
  );
}

function ProfileDialog({
  open,
  onOpenChange,
  profile,
  saving,
  onSave,
  t,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  profile: ProfileFormData | null;
  saving: boolean;
  onSave: (form: ProfileFormData) => Promise<void>;
  t: (key: string) => string;
}) {
  const [form, setForm] = useState<ProfileFormData | null>(null);

  useEffect(() => {
    if (profile) setForm({ ...profile });
  }, [profile]);

  if (!form) return null;

  const update = <K extends keyof ProfileFormData>(
    key: K,
    value: ProfileFormData[K],
  ) => setForm((f) => (f ? { ...f, [key]: value } : f));

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {form.isNew ? t("dialog.addTitle") : t("dialog.editTitle")}
          </DialogTitle>
          <DialogDescription>{t("dialog.description")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label>{t("dialog.name")}</Label>
            <Input
              value={form.name}
              onChange={(e) => update("name", e.target.value)}
              placeholder="shopee"
              disabled={!form.isNew}
              className="text-base md:text-sm"
            />
          </div>

          <div className="space-y-2">
            <Label>{t("dialog.remoteUrl")}</Label>
            <Input
              value={form.remote_url}
              onChange={(e) => update("remote_url", e.target.value)}
              placeholder="ws://cloak-shopee:9222"
              className="text-base md:text-sm"
            />
            <p className="text-xs text-muted-foreground">
              {t("dialog.remoteUrlHint")}
            </p>
          </div>

          <div className="flex items-center justify-between">
            <div>
              <Label>{t("dialog.shared")}</Label>
              <p className="text-xs text-muted-foreground">
                {t("dialog.sharedHint")}
              </p>
            </div>
            <Switch
              checked={form.shared}
              onCheckedChange={(v) => update("shared", v)}
            />
          </div>

          <div className="space-y-2">
            <Label>{t("dialog.domains")}</Label>
            <Input
              value={form.domains}
              onChange={(e) => update("domains", e.target.value)}
              placeholder="shopee.*, *.shopee.*"
              className="text-base md:text-sm"
            />
            <p className="text-xs text-muted-foreground">
              {t("dialog.domainsHint")}
            </p>
          </div>

          <div className="space-y-2">
            <Label>{t("dialog.vncUrl")}</Label>
            <Input
              value={form.vnc_url}
              onChange={(e) => update("vnc_url", e.target.value)}
              placeholder="http://103.97.126.134:6080"
              className="text-base md:text-sm"
            />
            <p className="text-xs text-muted-foreground">
              {t("dialog.vncUrlHint")}
            </p>
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div className="space-y-2">
              <Label>{t("dialog.maxPages")}</Label>
              <Input
                type="number"
                value={form.max_pages}
                onChange={(e) => update("max_pages", Number(e.target.value))}
                className="text-base md:text-sm"
              />
            </div>
            <div className="space-y-2">
              <Label>{t("dialog.actionTimeout")}</Label>
              <Input
                type="number"
                value={form.action_timeout_ms}
                onChange={(e) =>
                  update("action_timeout_ms", Number(e.target.value))
                }
                className="text-base md:text-sm"
              />
            </div>
            <div className="space-y-2">
              <Label>{t("dialog.idleTimeout")}</Label>
              <Input
                type="number"
                value={form.idle_timeout_ms}
                onChange={(e) =>
                  update("idle_timeout_ms", Number(e.target.value))
                }
                className="text-base md:text-sm"
              />
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("dialog.cancel")}
          </Button>
          <Button
            onClick={() => onSave(form)}
            disabled={!form.name || saving}
          >
            {saving ? t("dialog.saving") : t("dialog.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
