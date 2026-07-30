import { useState } from "react";
import { useLocation, useNavigate } from "react-router";
import { useTranslation } from "react-i18next";
import { ShieldAlert, LogOut, Plus, Trash2 } from "lucide-react";
import { useAuthStore } from "@/stores/use-auth-store";
import { useWs } from "@/hooks/use-ws";
import { Methods } from "@/api/protocol";
import { LoginLayout } from "./login-layout";
import { ROUTES, LOCAL_STORAGE_KEYS } from "@/lib/constants";

export function TenantSelectorPage() {
  const { t } = useTranslation("login");
  const location = useLocation();
  const navigate = useNavigate();
  const ws = useWs();
  const availableTenants = useAuthStore((s) => s.availableTenants);
  const isOwner = useAuthStore((s) => s.isOwner);
  const logout = useAuthStore((s) => s.logout);

  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [creating, setCreating] = useState(false);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [deleteName, setDeleteName] = useState("");
  const [deleting, setDeleting] = useState(false);

  const from = (location.state as { from?: { pathname: string } })?.from?.pathname;

  const handleSelect = (tenantSlug: string) => {
    localStorage.setItem(LOCAL_STORAGE_KEYS.TENANT_ID, tenantSlug);
    useAuthStore.getState().setTenantSelected(true);
    window.location.replace(from || ROUTES.OVERVIEW);
  };

  const handleLogout = () => {
    logout();
    navigate(ROUTES.LOGIN, { replace: true });
  };

  const handleNameChange = (v: string) => {
    setName(v);
    setSlug(v.toLowerCase().replace(/\s+/g, "-").replace(/[^a-z0-9-]/g, ""));
  };

  const handleCreate = async () => {
    if (!name.trim() || !slug.trim()) return;
    setCreating(true);
    try {
      await ws.call(Methods.TENANTS_CREATE, { name: name.trim(), slug: slug.trim() });
      setCreateOpen(false);
      setName("");
      setSlug("");
      window.location.reload();
    } catch {
      // Error handled by socket layer
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteId) return;
    setDeleting(true);
    try {
      await ws.call(Methods.TENANTS_DELETE, { id: deleteId });
      setDeleteId(null);
      window.location.reload();
    } catch {
      // Error handled by socket layer
    } finally {
      setDeleting(false);
    }
  };

  const isMasterTenant = (id: string) => id === "0193a5b0-7000-7000-8000-000000000001";

  // No access state: not owner and no tenants
  if (!isOwner && availableTenants.length === 0) {
    return (
      <LoginLayout subtitle={t("noAccess")}>
        <div className="space-y-5 text-center">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-amber-100 dark:bg-amber-950/40">
            <ShieldAlert className="h-7 w-7 text-amber-600 dark:text-amber-400" />
          </div>
          <div className="space-y-2">
            <p className="text-sm text-muted-foreground">{t("noAccessDescription")}</p>
            <p className="text-xs text-muted-foreground/70">{t("noAccessHint")}</p>
          </div>
          <button
            onClick={handleLogout}
            className="inline-flex w-full items-center justify-center gap-2 rounded-md border border-input bg-background px-4 py-2.5 text-base md:text-sm font-medium hover:bg-muted transition-colors"
          >
            <LogOut className="h-4 w-4" />
            {t("logout")}
          </button>
        </div>
      </LoginLayout>
    );
  }

  return (
    <LoginLayout subtitle={t("selectTenantDescription")}>
      <div className="space-y-3">
        <h2 className="text-center text-base font-medium">{t("selectTenant")}</h2>

        {/* Tenant cards */}
        {availableTenants.map((tenant) => (
          <div
            key={tenant.id}
            className="group relative w-full rounded-lg border border-input bg-card transition-colors hover:bg-muted"
          >
            <button
              onClick={() => handleSelect(tenant.slug)}
              className="w-full p-4 text-left"
            >
              <div className="flex items-center justify-between gap-2">
                <div className="min-w-0">
                  <p className="truncate font-medium">{tenant.name}</p>
                  <p className="mt-0.5 truncate text-xs text-muted-foreground">{tenant.slug}</p>
                </div>
                <span className="shrink-0 rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground capitalize">
                  {tenant.role}
                </span>
              </div>
            </button>
            {/* Delete button - only for owner, never for Master tenant */}
            {isOwner && !isMasterTenant(tenant.id) && (
              <button
                onClick={(e) => { e.stopPropagation(); setDeleteId(tenant.id); setDeleteName(tenant.name); }}
                className="absolute right-2 top-2 hidden rounded-md p-1.5 text-muted-foreground/50 hover:bg-destructive/10 hover:text-destructive group-hover:block"
                title={t("deleteTenant")}
              >
                <Trash2 className="h-3.5 w-3.5" />
              </button>
            )}
          </div>
        ))}

        {/* Create button - only for owner */}
        {isOwner && (
          <button
            onClick={() => setCreateOpen(true)}
            className="inline-flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-input bg-background px-4 py-3 text-base md:text-sm font-medium text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
          >
            <Plus className="h-4 w-4" />
            {t("createTenant")}
          </button>
        )}
      </div>

      {/* Create dialog */}
      {createOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setCreateOpen(false)}>
          <div className="mx-4 w-full max-w-sm rounded-lg border bg-card p-6 shadow-lg" onClick={(e) => e.stopPropagation()}>
            <h3 className="text-lg font-semibold">{t("createTenant")}</h3>
            <p className="mt-1 text-sm text-muted-foreground">{t("createTenantDescription")}</p>
            <div className="mt-4 space-y-3">
              <div>
                <label className="text-sm font-medium">{t("tenantName")}</label>
                <input
                  value={name}
                  onChange={(e) => handleNameChange(e.target.value)}
                  placeholder={t("tenantName")}
                  className="mt-1 w-full rounded-md border border-input bg-background px-3 py-2 text-base md:text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                  autoFocus
                />
              </div>
              <div>
                <label className="text-sm font-medium">{t("tenantSlug")}</label>
                <input
                  value={slug}
                  onChange={(e) => setSlug(e.target.value)}
                  placeholder="my-org"
                  className="mt-1 w-full rounded-md border border-input bg-background px-3 py-2 text-base md:text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                />
                <p className="mt-1 text-xs text-muted-foreground">{t("tenantSlugHelp")}</p>
              </div>
            </div>
            <div className="mt-5 flex justify-end gap-2">
              <button
                onClick={() => setCreateOpen(false)}
                className="rounded-md border border-input px-4 py-2 text-sm hover:bg-muted"
                disabled={creating}
              >
                {t("cancel", { ns: "common" })}
              </button>
              <button
                onClick={handleCreate}
                disabled={creating || !name.trim() || !slug.trim()}
                className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
              >
                {creating ? "..." : t("createTenant")}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete confirmation dialog */}
      {deleteId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setDeleteId(null)}>
          <div className="mx-4 w-full max-w-sm rounded-lg border bg-card p-6 shadow-lg" onClick={(e) => e.stopPropagation()}>
            <h3 className="text-lg font-semibold text-destructive">{t("deleteTenant")}</h3>
            <p className="mt-2 text-sm text-muted-foreground">
              {t("confirmDeleteTenant", { name: deleteName })}
            </p>
            <div className="mt-5 flex justify-end gap-2">
              <button
                onClick={() => setDeleteId(null)}
                className="rounded-md border border-input px-4 py-2 text-sm hover:bg-muted"
                disabled={deleting}
              >
                {t("cancel", { ns: "common" })}
              </button>
              <button
                onClick={handleDelete}
                disabled={deleting}
                className="rounded-md bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground hover:bg-destructive/90 disabled:opacity-50"
              >
                {deleting ? "..." : t("deleteTenant")}
              </button>
            </div>
          </div>
        </div>
      )}
    </LoginLayout>
  );
}
