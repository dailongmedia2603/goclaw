import { useTranslation } from "react-i18next";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { useGetPlan } from "@/hooks/use-fbcloak";

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

export interface PlanDetailDialogProps {
  planID: string | null;
  open: boolean;
  onClose: () => void;
}

export function PlanDetailDialog({ planID, open, onClose }: PlanDetailDialogProps) {
  const { t } = useTranslation("fbcloak");
  const { data, isLoading } = useGetPlan(planID);
  const plan = data?.plan;

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-2xl max-sm:inset-0 max-sm:max-w-full max-sm:rounded-none">
        <DialogHeader>
          <DialogTitle>{t("plans.detail.title")}</DialogTitle>
        </DialogHeader>
        {isLoading || !plan ? (
          <div className="py-6 text-center text-sm text-muted-foreground">
            {t("plans.loading")}
          </div>
        ) : (
          <div className="space-y-3 text-sm">
            <Row label={t("plans.detail.recipientLabel")}>
              <div>{plan.recipientName || plan.psid}</div>
              <div className="text-xs text-muted-foreground">{plan.psid}</div>
            </Row>
            <Row label={t("plans.cols.status")}>
              <Badge
                variant="outline"
                className={STATUS_CLASSES[plan.status] ?? STATUS_CLASSES.cancelled}
              >
                {t(`plans.status.${camelStatus(plan.status)}`)}
              </Badge>
            </Row>
            <Row label={t("plans.detail.scheduledLabel")}>
              {new Date(plan.scheduledAt).toLocaleString()}
            </Row>
            <Row label={t("plans.detail.messageLabel")}>
              <div className="rounded-md bg-muted p-3 whitespace-pre-wrap">{plan.messageDraft}</div>
            </Row>
            <Row label={t("plans.detail.reasonLabel")}>{plan.reason || "—"}</Row>
            {plan.skipReason && (
              <Row label={t("plans.detail.skipReasonLabel")}>{plan.skipReason}</Row>
            )}
            <Row label={t("plans.detail.modelLabel")}>{plan.generatedByModel || "—"}</Row>
            <Row label={t("plans.detail.generatedAtLabel")}>
              {new Date(plan.generatedAt).toLocaleString()}
            </Row>
            {plan.sentAt && (
              <Row label={t("plans.detail.sentAtLabel")}>
                {new Date(plan.sentAt).toLocaleString()}
              </Row>
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-[140px_1fr] gap-1 sm:gap-3">
      <div className="text-muted-foreground">{label}</div>
      <div className="break-words">{children}</div>
    </div>
  );
}
