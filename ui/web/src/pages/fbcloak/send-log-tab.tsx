import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { EmptyState } from "@/components/shared/empty-state";
import {
  type FBCloakSendLog,
  type SendStatus,
  useFBCloakJobs,
  useFBCloakSendLog,
} from "@/hooks/use-fbcloak";
import { ScreenshotViewer } from "./screenshot-viewer";

const STATUS_OPTIONS: SendStatus[] = ["sent", "dry_run", "skipped", "failed"];

export function SendLogTab() {
  const { t } = useTranslation("fbcloak");
  const { data: jobs = [] } = useFBCloakJobs();

  const [jobId, setJobId] = useState<string>("");
  const [status, setStatus] = useState<string>("");
  const [fromDate, setFromDate] = useState<string>("");
  const [toDate, setToDate] = useState<string>("");
  const [activeFilters, setActiveFilters] = useState<{
    jobId?: string;
    status?: SendStatus;
    fromDate?: string;
    toDate?: string;
  }>({});

  const { data: logs = [] } = useFBCloakSendLog(activeFilters);
  const [detail, setDetail] = useState<FBCloakSendLog | null>(null);

  const apply = () => {
    setActiveFilters({
      jobId: jobId || undefined,
      status: (status as SendStatus) || undefined,
      fromDate: fromDate ? new Date(fromDate).toISOString() : undefined,
      toDate: toDate ? new Date(toDate).toISOString() : undefined,
    });
  };
  const clear = () => {
    setJobId("");
    setStatus("");
    setFromDate("");
    setToDate("");
    setActiveFilters({});
  };

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3 rounded-md border p-3">
        <div className="grid gap-1">
          <Label>{t("sendLog.filters.job")}</Label>
          <Select value={jobId || "_all"} onValueChange={(v) => setJobId(v === "_all" ? "" : v)}>
            <SelectTrigger>
              <SelectValue placeholder={t("sendLog.filters.all")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="_all">{t("sendLog.filters.all")}</SelectItem>
              {jobs.map((j) => (
                <SelectItem key={j.id} value={j.id}>
                  {j.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="grid gap-1">
          <Label>{t("sendLog.filters.status")}</Label>
          <Select value={status || "_all"} onValueChange={(v) => setStatus(v === "_all" ? "" : v)}>
            <SelectTrigger>
              <SelectValue placeholder={t("sendLog.filters.all")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="_all">{t("sendLog.filters.all")}</SelectItem>
              {STATUS_OPTIONS.map((s) => (
                <SelectItem key={s} value={s}>
                  {t(`sendLog.status.${s}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="grid gap-1">
          <Label>{t("sendLog.filters.fromDate")}</Label>
          <Input
            type="date"
            value={fromDate}
            onChange={(e) => setFromDate(e.target.value)}
            className="text-base md:text-sm"
          />
        </div>
        <div className="grid gap-1">
          <Label>{t("sendLog.filters.toDate")}</Label>
          <Input
            type="date"
            value={toDate}
            onChange={(e) => setToDate(e.target.value)}
            className="text-base md:text-sm"
          />
        </div>
        <div className="flex items-end gap-2">
          <Button onClick={apply}>{t("sendLog.filters.apply")}</Button>
          <Button variant="outline" onClick={clear}>
            {t("sendLog.filters.clear")}
          </Button>
        </div>
      </div>

      {logs.length === 0 ? (
        <EmptyState title={t("sendLog.title")} description={t("sendLog.empty")} />
      ) : (
        <div className="overflow-x-auto rounded-md border">
          <table className="min-w-[600px] w-full text-sm">
            <thead className="bg-muted/50 text-left">
              <tr>
                <th className="px-3 py-2">{t("sendLog.columns.sentAt")}</th>
                <th className="px-3 py-2">{t("sendLog.columns.recipient")}</th>
                <th className="px-3 py-2">{t("sendLog.columns.status")}</th>
                <th className="px-3 py-2">{t("sendLog.columns.preview")}</th>
                <th className="px-3 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {logs.map((l) => (
                <tr key={l.id} className="border-t">
                  <td className="px-3 py-2 whitespace-nowrap text-xs">
                    {new Date(l.sentAt).toLocaleString()}
                  </td>
                  <td className="px-3 py-2">
                    <div>{l.recipientName ?? "—"}</div>
                    <div className="text-xs text-muted-foreground">{l.recipientPsid}</div>
                  </td>
                  <td className="px-3 py-2">
                    <SendStatusBadge status={l.status} />
                  </td>
                  <td className="px-3 py-2 max-w-md truncate text-xs">{l.messageText}</td>
                  <td className="px-3 py-2">
                    <Button variant="ghost" size="sm" onClick={() => setDetail(l)}>
                      {t("sendLog.row.openDetail")}
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Dialog open={!!detail} onOpenChange={(o) => !o && setDetail(null)}>
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t("sendLog.detail.title")}</DialogTitle>
          </DialogHeader>
          {detail && (
            <div className="grid gap-3 text-sm">
              <div>
                <Label>{t("sendLog.columns.sentAt")}</Label>
                <div>{new Date(detail.sentAt).toLocaleString()}</div>
              </div>
              <div>
                <Label>{t("sendLog.columns.status")}</Label>
                <div>
                  <SendStatusBadge status={detail.status} />
                </div>
              </div>
              <div>
                <Label>{t("sendLog.detail.messageText")}</Label>
                <pre className="whitespace-pre-wrap rounded bg-muted p-2 text-xs">
                  {detail.messageText}
                </pre>
              </div>
              {detail.skipReason && (
                <div>
                  <Label>{t("sendLog.detail.skipReason")}</Label>
                  <div className="text-xs text-muted-foreground">{detail.skipReason}</div>
                </div>
              )}
              {detail.error && (
                <div>
                  <Label>{t("sendLog.detail.errorReason")}</Label>
                  <div className="text-xs text-destructive">{detail.error}</div>
                </div>
              )}
              {detail.screenshotPre && (
                <div>
                  <Label>{t("sendLog.detail.screenshotPre")}</Label>
                  <ScreenshotViewer sendLogId={detail.id} kind="pre" />
                </div>
              )}
              {detail.screenshotPost && (
                <div>
                  <Label>{t("sendLog.detail.screenshotPost")}</Label>
                  <ScreenshotViewer sendLogId={detail.id} kind="post" />
                </div>
              )}
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setDetail(null)}>
              {t("sendLog.detail.close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function SendStatusBadge({ status }: { status: SendStatus }) {
  const { t } = useTranslation("fbcloak");
  const variant: "default" | "secondary" | "outline" | "destructive" =
    status === "sent" ? "default"
    : status === "failed" ? "destructive"
    : "secondary";
  return <Badge variant={variant}>{t(`sendLog.status.${status}`)}</Badge>;
}
