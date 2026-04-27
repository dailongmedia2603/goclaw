import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Sparkles, Play, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { EmptyState } from "@/components/shared/empty-state";
import {
  type FBCloakPlan,
  useFBCloakCredentials,
  useListPlans,
  usePlanStats,
  useGenerateNow,
  useCancelPlan,
  useRunDuePlans,
} from "@/hooks/use-fbcloak";
import { PlanDetailDialog } from "./plan-detail-dialog";

const STATUS_CLASSES: Record<string, string> = {
  pending: "bg-blue-500/20 text-blue-400 border-blue-500/40",
  sent: "bg-green-500/20 text-green-400 border-green-500/40",
  replan_needed: "bg-amber-500/20 text-amber-400 border-amber-500/40",
  skipped: "bg-zinc-500/20 text-zinc-400 border-zinc-500/40",
  cancelled: "bg-red-500/20 text-red-400 border-red-500/40",
  superseded: "bg-purple-500/20 text-purple-400 border-purple-500/40",
};

function camelStatus(s: string): string {
  return s.replace(/_([a-z])/g, (_, c) => c.toUpperCase());
}

export function PlansTab() {
  const { t } = useTranslation("fbcloak");
  const [credFilter, setCredFilter] = useState<string>("all");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [selectedPlanID, setSelectedPlanID] = useState<string | null>(null);
  const [confirmGenerate, setConfirmGenerate] = useState(false);
  const [confirmRunDue, setConfirmRunDue] = useState(false);
  const [cancelTarget, setCancelTarget] = useState<FBCloakPlan | null>(null);

  const { data: credentialsData = [] } = useFBCloakCredentials();
  const { data: statsData } = usePlanStats();
  const { data: plansData, isLoading } = useListPlans({
    status: statusFilter === "all" ? undefined : [statusFilter],
    credentialId: credFilter === "all" ? undefined : credFilter,
    limit: 100,
  });
  const generateM = useGenerateNow();
  const cancelM = useCancelPlan();
  const runDueM = useRunDuePlans();

  const stats = statsData?.stats;
  const plans = plansData?.plans ?? [];

  const onGenerate = () => {
    if (credFilter === "all") {
      alert(t("plans.credentialRequired"));
      return;
    }
    setConfirmGenerate(true);
  };

  return (
    <div className="space-y-4 py-4">
      <p className="text-sm text-muted-foreground">{t("plans.description")}</p>

      {/* Stats row */}
      <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
        {(["pending", "sent", "replanNeeded", "skipped", "cancelled"] as const).map((k) => (
          <div key={k} className="rounded-md border bg-card p-3">
            <div className="text-xs text-muted-foreground">{t(`plans.stats.${k}`)}</div>
            <div className="text-2xl font-semibold tabular-nums">
              {stats ? (stats as unknown as Record<string, number>)[k] ?? 0 : "—"}
            </div>
          </div>
        ))}
      </div>

      {/* Filter + actions */}
      <div className="flex flex-col sm:flex-row gap-2 items-stretch sm:items-center">
        <select
          value={credFilter}
          onChange={(e) => setCredFilter(e.target.value)}
          className="text-base md:text-sm rounded-md border bg-background px-3 py-2 sm:w-[260px]"
        >
          <option value="all">{t("plans.filters.allCredentials")}</option>
          {credentialsData.map((c) => (
            <option key={c.id} value={c.id}>
              {c.fanpageName}
            </option>
          ))}
        </select>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="text-base md:text-sm rounded-md border bg-background px-3 py-2 sm:w-[180px]"
        >
          <option value="all">{t("plans.filters.allStatus")}</option>
          <option value="pending">{t("plans.status.pending")}</option>
          <option value="sent">{t("plans.status.sent")}</option>
          <option value="replan_needed">{t("plans.status.replanNeeded")}</option>
          <option value="skipped">{t("plans.status.skipped")}</option>
          <option value="cancelled">{t("plans.status.cancelled")}</option>
        </select>
        <div className="flex gap-2 sm:ml-auto">
          <Button
            variant="outline"
            disabled={runDueM.isPending}
            onClick={() => setConfirmRunDue(true)}
          >
            <Play className="mr-1 h-4 w-4" />
            {t("plans.actions.runDue")}
          </Button>
          <Button disabled={generateM.isPending} onClick={onGenerate}>
            <Sparkles className="mr-1 h-4 w-4" />
            {generateM.isPending ? t("plans.actions.generating") : t("plans.actions.generateNow")}
          </Button>
        </div>
      </div>

      {/* Table */}
      <div className="overflow-x-auto rounded-md border">
        <table className="w-full min-w-[600px] text-sm">
          <thead className="bg-muted/50 text-left">
            <tr>
              <th className="px-3 py-2 font-medium">{t("plans.cols.recipient")}</th>
              <th className="px-3 py-2 font-medium">{t("plans.cols.scheduledAt")}</th>
              <th className="px-3 py-2 font-medium">{t("plans.cols.status")}</th>
              <th className="px-3 py-2 font-medium">{t("plans.cols.message")}</th>
              <th className="px-3 py-2 font-medium text-right">{t("plans.cols.actions")}</th>
            </tr>
          </thead>
          <tbody>
            {isLoading && (
              <tr>
                <td colSpan={5} className="py-6 text-center text-muted-foreground">
                  {t("plans.loading")}
                </td>
              </tr>
            )}
            {!isLoading && plans.length === 0 && (
              <tr>
                <td colSpan={5}>
                  <EmptyState title={t("plans.empty")} />
                </td>
              </tr>
            )}
            {plans.map((p) => (
              <tr
                key={p.id}
                className="border-t hover:bg-muted/30 cursor-pointer"
                onClick={() => setSelectedPlanID(p.id)}
              >
                <td className="px-3 py-3">
                  <div className="font-medium">{p.recipientName || p.psid}</div>
                  <div className="text-xs text-muted-foreground">{p.psid}</div>
                </td>
                <td className="px-3 py-3 text-sm whitespace-nowrap">
                  {new Date(p.scheduledAt).toLocaleString()}
                </td>
                <td className="px-3 py-3">
                  <Badge
                    variant="outline"
                    className={STATUS_CLASSES[p.status] ?? STATUS_CLASSES.cancelled}
                  >
                    {t(`plans.status.${camelStatus(p.status)}`)}
                  </Badge>
                </td>
                <td className="px-3 py-3 max-w-[300px] truncate">{p.messageDraft}</td>
                <td className="px-3 py-3 text-right">
                  {(p.status === "pending" || p.status === "replan_needed") && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={(e) => {
                        e.stopPropagation();
                        setCancelTarget(p);
                      }}
                    >
                      <X className="h-4 w-4" />
                      <span className="sr-only">{t("plans.actions.cancel")}</span>
                    </Button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <PlanDetailDialog
        planID={selectedPlanID}
        open={!!selectedPlanID}
        onClose={() => setSelectedPlanID(null)}
      />

      <ConfirmDialog
        open={confirmGenerate}
        onOpenChange={setConfirmGenerate}
        title={t("plans.actions.generateNow")}
        description={t("plans.confirmGenerate")}
        onConfirm={() => {
          if (credFilter !== "all") generateM.mutate(credFilter);
        }}
      />

      <ConfirmDialog
        open={confirmRunDue}
        onOpenChange={setConfirmRunDue}
        title={t("plans.actions.runDue")}
        description={t("plans.confirmRunDue")}
        onConfirm={() => runDueM.mutate()}
      />

      <ConfirmDialog
        open={!!cancelTarget}
        onOpenChange={(open) => !open && setCancelTarget(null)}
        title={t("plans.actions.cancel")}
        description={t("plans.confirmCancel")}
        onConfirm={() => {
          if (cancelTarget) cancelM.mutate(cancelTarget.id);
          setCancelTarget(null);
        }}
      />
    </div>
  );
}
