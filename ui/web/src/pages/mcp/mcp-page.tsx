import { useState, useEffect, lazy, Suspense, useRef } from "react";
import { useTranslation } from "react-i18next";
import { Plug, Plus, RefreshCw, RotateCcw, Pencil, Trash2, Users, Wrench, KeyRound, ChevronDown, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { SearchInput } from "@/components/shared/search-input";
import { Pagination } from "@/components/shared/pagination";
import { TableSkeleton } from "@/components/shared/loading-skeleton";
import { ConfirmDeleteDialog } from "@/components/shared/confirm-delete-dialog";
import { useMCP, type MCPServerData, type MCPServerInput } from "./hooks/use-mcp";
import { MCPFormDialog } from "./mcp-form-dialog";
import { MCPToolsDialog } from "./mcp-tools-dialog";
import { PresetsCatalogDialog } from "./presets/presets-catalog-dialog";
import { PresetFormDispatcher } from "./presets/preset-form-dispatcher";
import { getServerPreset } from "./presets/use-presets";
import { useMinLoading } from "@/hooks/use-min-loading";
import { useDeferredLoading } from "@/hooks/use-deferred-loading";
import { usePagination } from "@/hooks/use-pagination";

const MCPGrantsDialog = lazy(() =>
  import("./mcp-grants-dialog").then((m) => ({ default: m.MCPGrantsDialog }))
);
const MCPUserCredentialsDialog = lazy(() =>
  import("./mcp-user-credentials-dialog").then((m) => ({ default: m.MCPUserCredentialsDialog }))
);

const transportBadge: Record<string, string> = {
  stdio: "default",
  sse: "secondary",
  "streamable-http": "outline",
};

export function MCPPage() {
  const { t } = useTranslation("mcp");
  const { t: tc } = useTranslation("common");
  const {
    servers,
    loading,
    fetching,
    refresh,
    createServer,
    updateServer,
    deleteServer,
    grantAgent,
    revokeAgent,
    listAgentGrants,
    testConnection,
    reconnectServer,
    listServerTools,
    getUserCredentials,
    setUserCredentials,
    deleteUserCredentials,
  } = useMCP();
  const spinning = useMinLoading(fetching);
  const showSkeleton = useDeferredLoading(loading && servers.length === 0);
  const [search, setSearch] = useState("");

  // Custom (generic) form state
  const [formOpen, setFormOpen] = useState(false);
  const [editServer, setEditServer] = useState<MCPServerData | null>(null);

  // Preset catalog + preset-specific form state
  const [catalogOpen, setCatalogOpen] = useState(false);
  const [presetFormOpen, setPresetFormOpen] = useState(false);
  const [activePresetId, setActivePresetId] = useState<string | null>(null);
  const [activePresetServer, setActivePresetServer] = useState<MCPServerData | null>(null);

  // Add-server dropdown
  const [addMenuOpen, setAddMenuOpen] = useState(false);
  const addMenuRef = useRef<HTMLDivElement>(null);

  const [grantsServer, setGrantsServer] = useState<MCPServerData | null>(null);
  const [toolsServer, setToolsServer] = useState<MCPServerData | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<MCPServerData | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);
  const [credentialsServer, setCredentialsServer] = useState<MCPServerData | null>(null);
  const [reconnectingId, setReconnectingId] = useState<string | null>(null);

  const filtered = servers.filter(
    (s) =>
      s.name.toLowerCase().includes(search.toLowerCase()) ||
      (s.display_name || "").toLowerCase().includes(search.toLowerCase()),
  );

  const { pageItems, pagination, setPage, setPageSize, resetPage } = usePagination(filtered);

  useEffect(() => { resetPage(); }, [search, resetPage]);

  // Close add-menu when clicking outside
  useEffect(() => {
    if (!addMenuOpen) return;
    const onClick = (e: MouseEvent) => {
      if (!addMenuRef.current?.contains(e.target as Node)) setAddMenuOpen(false);
    };
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, [addMenuOpen]);

  const handleCreate = async (data: MCPServerInput) => {
    await createServer(data);
  };

  const handleEdit = async (data: MCPServerInput) => {
    if (!editServer) return;
    await updateServer(editServer.id, data);
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleteLoading(true);
    try {
      await deleteServer(deleteTarget.id);
      setDeleteTarget(null);
    } finally {
      setDeleteLoading(false);
    }
  };

  const openEditFor = (srv: MCPServerData) => {
    const preset = getServerPreset(srv);
    if (preset === "lark") {
      setActivePresetId(preset);
      setActivePresetServer(srv);
      setPresetFormOpen(true);
    } else {
      setEditServer(srv);
      setFormOpen(true);
    }
  };

  const openCreateCustom = () => {
    setEditServer(null);
    setFormOpen(true);
  };

  const handlePresetSelect = (presetId: string | "__custom__") => {
    if (presetId === "__custom__") {
      openCreateCustom();
      return;
    }
    setActivePresetId(presetId);
    setActivePresetServer(null);
    setPresetFormOpen(true);
  };

  return (
    <div className="p-4 sm:p-6 pb-10">
      <PageHeader
        title={t("title")}
        description={t("description")}
        actions={
          <div className="flex gap-2">
            <div className="relative" ref={addMenuRef}>
              <Button
                size="sm"
                onClick={() => setAddMenuOpen((o) => !o)}
                className="gap-1"
                data-testid="mcp-add-btn"
              >
                <Plus className="h-3.5 w-3.5" /> {t("addServer")}
                <ChevronDown className="h-3.5 w-3.5" />
              </Button>
              {addMenuOpen && (
                <div className="absolute right-0 top-full mt-1 min-w-[14rem] rounded-md border bg-popover text-popover-foreground shadow-md z-50">
                  <button
                    type="button"
                    onClick={() => {
                      setAddMenuOpen(false);
                      setCatalogOpen(true);
                    }}
                    className="flex w-full items-center gap-2 px-3 py-2 text-sm hover:bg-accent focus:bg-accent outline-none"
                    data-testid="mcp-add-preset"
                  >
                    <Sparkles className="h-4 w-4" />
                    <div className="flex-1 text-left">
                      <div className="font-medium">{t("addFromPreset")}</div>
                      <div className="text-xs text-muted-foreground">{t("addFromPresetHint")}</div>
                    </div>
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      setAddMenuOpen(false);
                      openCreateCustom();
                    }}
                    className="flex w-full items-center gap-2 px-3 py-2 text-sm hover:bg-accent focus:bg-accent outline-none border-t"
                    data-testid="mcp-add-custom"
                  >
                    <Wrench className="h-4 w-4" />
                    <div className="flex-1 text-left">
                      <div className="font-medium">{t("addCustom")}</div>
                      <div className="text-xs text-muted-foreground">{t("addCustomHint")}</div>
                    </div>
                  </button>
                </div>
              )}
            </div>
            <Button variant="outline" size="sm" onClick={refresh} disabled={spinning} className="gap-1">
              <RefreshCw className={"h-3.5 w-3.5" + (spinning ? " animate-spin" : "")} /> {tc("refresh")}
            </Button>
          </div>
        }
      />

      <div className="mt-4">
        <SearchInput
          value={search}
          onChange={setSearch}
          placeholder={t("searchPlaceholder")}
          className="max-w-sm"
        />
      </div>

      <div className="mt-4">
        {showSkeleton ? (
          <TableSkeleton rows={5} />
        ) : filtered.length === 0 ? (
          <EmptyState
            icon={Plug}
            title={search ? t("noMatchTitle") : t("emptyTitle")}
            description={search ? t("noMatchDescription") : t("emptyDescription")}
          />
        ) : (
          <div className="overflow-x-auto rounded-md border">
            <table className="w-full min-w-[600px] text-sm">
              <thead>
                <tr className="border-b bg-muted/50">
                  <th className="px-4 py-3 text-left font-medium">{t("columns.name")}</th>
                  <th className="px-4 py-3 text-left font-medium">{t("columns.transport")}</th>
                  <th className="px-4 py-3 text-center font-medium">{t("columns.tools")}</th>
                  <th className="px-4 py-3 text-center font-medium">{t("columns.agents")}</th>
                  <th className="px-4 py-3 text-left font-medium">{t("columns.enabled")}</th>
                  <th className="px-4 py-3 text-left font-medium">{t("columns.createdBy")}</th>
                  <th className="px-4 py-3 text-right font-medium">{t("columns.actions")}</th>
                </tr>
              </thead>
              <tbody>
                {pageItems.map((srv) => {
                  const preset = getServerPreset(srv);
                  return (
                    <tr key={srv.id} className="border-b last:border-0 hover:bg-muted/30">
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <Plug className="h-4 w-4 text-muted-foreground shrink-0 mt-0.5" />
                          <div>
                            <div className="flex items-center gap-2">
                              <span className="font-medium">{srv.display_name || srv.name}</span>
                              {srv.display_name && (
                                <span className="text-xs text-muted-foreground">({srv.name})</span>
                              )}
                              {preset && (
                                <Badge variant="outline" className="text-[10px] py-0 px-1.5 h-4" data-testid={`preset-badge-${preset}`}>
                                  <Sparkles className="h-2.5 w-2.5 mr-0.5" />
                                  {preset}
                                </Badge>
                              )}
                            </div>
                            <span className="font-mono text-xs-plus text-muted-foreground">
                              {srv.tool_prefix || `mcp_${srv.name.replace(/-/g, "_")}`}
                            </span>
                          </div>
                        </div>
                      </td>
                      <td className="px-4 py-3">
                        <Badge variant={transportBadge[srv.transport] as "default" | "secondary" | "outline" ?? "outline"}>
                          {srv.transport}
                        </Badge>
                      </td>
                      <td className="px-4 py-3 text-center">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setToolsServer(srv)}
                          title={t("viewTools")}
                        >
                          <Wrench className="h-3.5 w-3.5" />
                        </Button>
                      </td>
                      <td className="px-4 py-3 text-center text-muted-foreground">
                        {srv.agent_count ?? 0}
                      </td>
                      <td className="px-4 py-3">
                        <Badge variant={srv.enabled ? "default" : "secondary"}>
                          {srv.enabled ? tc("yes") : tc("no")}
                        </Badge>
                      </td>
                      <td className="px-4 py-3 text-muted-foreground">{srv.created_by || "-"}</td>
                      <td className="px-4 py-3 text-right">
                        <div className="flex items-center justify-end gap-1">
                          <Button
                            variant="ghost"
                            size="sm"
                            disabled={reconnectingId === srv.id}
                            onClick={async () => {
                              setReconnectingId(srv.id);
                              try { await reconnectServer(srv.id); } finally { setReconnectingId(null); }
                            }}
                            title={t("reconnect")}
                          >
                            <RotateCcw className={"h-3.5 w-3.5" + (reconnectingId === srv.id ? " animate-spin" : "")} />
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => setGrantsServer(srv)}
                            className="gap-1"
                            title={t("manageGrants")}
                          >
                            <Users className="h-3.5 w-3.5" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => setCredentialsServer(srv)}
                            title={t("userCredentials.title")}
                          >
                            <KeyRound className="h-3.5 w-3.5" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => openEditFor(srv)}
                            className="gap-1"
                          >
                            <Pencil className="h-3.5 w-3.5" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => setDeleteTarget(srv)}
                            className="gap-1 text-destructive hover:text-destructive"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            <Pagination
              page={pagination.page}
              pageSize={pagination.pageSize}
              total={pagination.total}
              totalPages={pagination.totalPages}
              onPageChange={setPage}
              onPageSizeChange={setPageSize}
            />
          </div>
        )}
      </div>

      <MCPFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        server={editServer}
        onSubmit={editServer ? handleEdit : handleCreate}
        onTest={testConnection}
      />

      <PresetsCatalogDialog
        open={catalogOpen}
        onOpenChange={setCatalogOpen}
        onSelect={handlePresetSelect}
      />

      <PresetFormDispatcher
        presetId={activePresetId}
        server={activePresetServer}
        open={presetFormOpen}
        onOpenChange={(o) => {
          setPresetFormOpen(o);
          if (!o) {
            setActivePresetId(null);
            setActivePresetServer(null);
          }
        }}
      />

      {grantsServer && (
        <Suspense fallback={null}>
          <MCPGrantsDialog
            open={!!grantsServer}
            onOpenChange={(open) => !open && setGrantsServer(null)}
            server={grantsServer}
            onGrant={(agentId, allow, deny) => grantAgent(grantsServer.id, agentId, allow, deny)}
            onRevoke={(agentId) => revokeAgent(grantsServer.id, agentId)}
            onLoadGrants={listAgentGrants}
            onLoadTools={listServerTools}
          />
        </Suspense>
      )}

      {toolsServer && (
        <MCPToolsDialog
          open={!!toolsServer}
          onOpenChange={(open) => !open && setToolsServer(null)}
          server={toolsServer}
          onLoadTools={listServerTools}
        />
      )}

      <ConfirmDeleteDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t("delete.title")}
        description={t("delete.description", { name: deleteTarget?.display_name || deleteTarget?.name })}
        confirmValue={deleteTarget?.display_name || deleteTarget?.name || ""}
        confirmLabel={t("delete.confirmLabel")}
        onConfirm={handleDelete}
        loading={deleteLoading}
      />

      {credentialsServer && (
        <Suspense fallback={null}>
          <MCPUserCredentialsDialog
            open={!!credentialsServer}
            onOpenChange={(open) => !open && setCredentialsServer(null)}
            server={credentialsServer}
            onGetCredentials={getUserCredentials}
            onSetCredentials={setUserCredentials}
            onDeleteCredentials={deleteUserCredentials}
          />
        </Suspense>
      )}
    </div>
  );
}
