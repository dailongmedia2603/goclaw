import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { AlertTriangle, Plus, Play, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { EmptyState } from "@/components/shared/empty-state";
import {
  type FBCloakJob,
  useDeleteJob,
  useDisclaimerStatus,
  useFBCloakCredentials,
  useFBCloakJobs,
  useRunJobNow,
  useToggleJob,
} from "@/hooks/use-fbcloak";
import { JobEditDialog } from "./job-edit-dialog";
import { DisclaimerModal } from "./disclaimer-modal";

export function JobsTab() {
  const { t } = useTranslation("fbcloak");
  const { data: jobs = [] } = useFBCloakJobs();
  const { data: credentials = [] } = useFBCloakCredentials();
  const toggleM = useToggleJob();
  const deleteM = useDeleteJob();
  const runM = useRunJobNow();

  const [showCreate, setShowCreate] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<FBCloakJob | null>(null);
  const [runTarget, setRunTarget] = useState<FBCloakJob | null>(null);
  const [showDisclaimer, setShowDisclaimer] = useState(false);
  const { data: disclaimer } = useDisclaimerStatus();
  const needAck = disclaimer?.required ?? false;

  // Auto-open disclaimer modal once per session when it's required and the
  // tenant has at least one credential set up (no point pestering before).
  useEffect(() => {
    if (needAck && credentials.length > 0) {
      setShowDisclaimer(true);
    }
  }, [needAck, credentials.length]);

  const credName = (id: string) =>
    credentials.find((c) => c.id === id)?.fanpageName ?? id.slice(0, 8);

  return (
    <div className="space-y-4">
      {needAck && (
        <Alert variant="destructive">
          <AlertTriangle className="h-4 w-4" />
          <AlertTitle>{t("disclaimer.title")}</AlertTitle>
          <AlertDescription className="flex items-center justify-between gap-2">
            <span>{t("disclaimer.needAck")}</span>
            <Button size="sm" onClick={() => setShowDisclaimer(true)}>
              {t("disclaimer.submit")}
            </Button>
          </AlertDescription>
        </Alert>
      )}
      <div className="flex justify-end">
        <Button onClick={() => setShowCreate(true)} disabled={credentials.length === 0}>
          <Plus className="h-4 w-4 mr-2" />
          {t("jobs.newButton")}
        </Button>
      </div>

      {jobs.length === 0 ? (
        <EmptyState title={t("jobs.title")} description={t("jobs.empty")} />
      ) : (
        <div className="overflow-x-auto rounded-md border">
          <table className="min-w-[600px] w-full text-sm">
            <thead className="bg-muted/50 text-left">
              <tr>
                <th className="px-3 py-2">{t("jobs.columns.name")}</th>
                <th className="px-3 py-2">{t("jobs.columns.credential")}</th>
                <th className="px-3 py-2">{t("jobs.columns.schedule")}</th>
                <th className="px-3 py-2">{t("jobs.columns.dryRun")}</th>
                <th className="px-3 py-2">{t("jobs.columns.enabled")}</th>
                <th className="px-3 py-2">{t("jobs.columns.lastRun")}</th>
                <th className="px-3 py-2">{t("jobs.columns.actions")}</th>
              </tr>
            </thead>
            <tbody>
              {jobs.map((j) => (
                <tr key={j.id} className="border-t">
                  <td className="px-3 py-2 font-medium">{j.name}</td>
                  <td className="px-3 py-2">{credName(j.credentialId)}</td>
                  <td className="px-3 py-2 font-mono text-xs">{j.cronExpr}</td>
                  <td className="px-3 py-2">
                    <Badge variant={j.dryRun ? "secondary" : "destructive"}>
                      {j.dryRun ? t("jobs.dryRunOn") : t("jobs.dryRunOff")}
                    </Badge>
                  </td>
                  <td className="px-3 py-2">
                    <Switch
                      checked={j.enabled}
                      onCheckedChange={(v) => {
                        if (v && needAck) {
                          setShowDisclaimer(true);
                          return;
                        }
                        toggleM.mutate({ id: j.id, enabled: v });
                      }}
                    />
                  </td>
                  <td className="px-3 py-2 text-xs">
                    {j.lastRunStatus ? (
                      <Badge variant={j.lastRunStatus === "ok" ? "default" : "secondary"}>
                        {t(`jobs.lastStatus.${j.lastRunStatus}`)}
                      </Badge>
                    ) : (
                      "—"
                    )}
                    <div className="text-muted-foreground mt-1">
                      {j.lastRunAt ? new Date(j.lastRunAt).toLocaleString() : ""}
                    </div>
                  </td>
                  <td className="px-3 py-2">
                    <div className="flex gap-1">
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={runM.isPending}
                        onClick={() => setRunTarget(j)}
                      >
                        <Play className="h-4 w-4 mr-1" />
                        {t("jobs.runNow")}
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => setDeleteTarget(j)}>
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && (
        <JobEditDialog
          open={showCreate}
          onClose={() => setShowCreate(false)}
          credentials={credentials}
        />
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
        title={t("jobs.delete")}
        description={t("jobs.confirmDelete")}
        confirmLabel={t("jobs.delete")}
        variant="destructive"
        onConfirm={async () => {
          if (deleteTarget) await deleteM.mutateAsync(deleteTarget.id);
          setDeleteTarget(null);
        }}
      />
      <ConfirmDialog
        open={!!runTarget}
        onOpenChange={(o) => !o && setRunTarget(null)}
        title={t("jobs.runNow")}
        description={t("jobs.runNowConfirm")}
        confirmLabel={t("jobs.runNow")}
        loading={runM.isPending}
        onConfirm={async () => {
          if (runTarget) await runM.mutateAsync(runTarget.id);
          setRunTarget(null);
        }}
      />
      <DisclaimerModal open={showDisclaimer} onOpenChange={setShowDisclaimer} />
    </div>
  );
}
