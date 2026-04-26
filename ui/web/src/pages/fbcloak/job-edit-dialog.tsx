import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { type FBCloakCredential, useCreateJob } from "@/hooks/use-fbcloak";

interface Props {
  open: boolean;
  onClose: () => void;
  credentials: FBCloakCredential[];
}

const ONE_DAY_SEC = 86_400;

export function JobEditDialog({ open, onClose, credentials }: Props) {
  const { t } = useTranslation("fbcloak");
  const createM = useCreateJob();

  const [name, setName] = useState("");
  const [credentialId, setCredentialId] = useState(credentials[0]?.id ?? "");
  const [cronExpr, setCronExpr] = useState("0 9 * * *");
  const [minIdleDays, setMinIdleDays] = useState(7);
  const [maxIdleDays, setMaxIdleDays] = useState(30);
  const [dailyCap, setDailyCap] = useState(10);
  const [whFrom, setWhFrom] = useState("08:00");
  const [whTo, setWhTo] = useState("21:00");
  const [whTz, setWhTz] = useState("Asia/Ho_Chi_Minh");
  const [useScannerFallback, setUseScannerFallback] = useState(false);

  const valid = useMemo(
    () =>
      name.trim() &&
      credentialId &&
      cronExpr.trim() &&
      minIdleDays > 0 &&
      maxIdleDays > minIdleDays &&
      dailyCap > 0 &&
      dailyCap <= 50,
    [name, credentialId, cronExpr, minIdleDays, maxIdleDays, dailyCap],
  );

  const handleSubmit = async () => {
    await createM.mutateAsync({
      credentialId,
      name: name.trim(),
      cronExpr: cronExpr.trim(),
      targetMinIdleSec: minIdleDays * ONE_DAY_SEC,
      targetMaxIdleSec: maxIdleDays * ONE_DAY_SEC,
      dailyCap,
      workingHours: { start: whFrom, end: whTo, tz: whTz },
      useScannerFallback,
    });
    onClose();
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("jobs.edit_dialog.createTitle")}</DialogTitle>
        </DialogHeader>
        <div className="grid gap-3">
          <Field label={t("jobs.edit_dialog.name")}>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="text-base md:text-sm"
            />
          </Field>
          <Field label={t("jobs.edit_dialog.credential")}>
            <Select value={credentialId} onValueChange={setCredentialId}>
              <SelectTrigger>
                <SelectValue placeholder={t("jobs.edit_dialog.credentialPlaceholder")} />
              </SelectTrigger>
              <SelectContent>
                {credentials.map((c) => (
                  <SelectItem key={c.id} value={c.id}>
                    {c.fanpageName} ({c.fanpageId})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          <Field label={t("jobs.edit_dialog.cronExpr")} hint={t("jobs.edit_dialog.cronHint")}>
            <Input
              value={cronExpr}
              onChange={(e) => setCronExpr(e.target.value)}
              className="text-base md:text-sm font-mono"
            />
          </Field>
          <div className="rounded border border-dashed p-2 text-xs text-muted-foreground">
            {minIdleDays > 7
              ? t("dualMode.cloakReason")
              : t("dualMode.graphReason")}
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <Field label={t("jobs.edit_dialog.minIdle")}>
              <Input
                type="number"
                min={1}
                value={minIdleDays}
                onChange={(e) => setMinIdleDays(Number(e.target.value))}
                className="text-base md:text-sm"
              />
            </Field>
            <Field label={t("jobs.edit_dialog.maxIdle")}>
              <Input
                type="number"
                min={2}
                value={maxIdleDays}
                onChange={(e) => setMaxIdleDays(Number(e.target.value))}
                className="text-base md:text-sm"
              />
            </Field>
          </div>
          <Field label={t("jobs.edit_dialog.dailyCap")} hint={t("jobs.edit_dialog.dailyCapHint")}>
            <Input
              type="number"
              min={1}
              max={50}
              value={dailyCap}
              onChange={(e) => setDailyCap(Number(e.target.value))}
              className="text-base md:text-sm"
            />
          </Field>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <Field label={t("jobs.edit_dialog.from")}>
              <Input
                type="time"
                value={whFrom}
                onChange={(e) => setWhFrom(e.target.value)}
                className="text-base md:text-sm"
              />
            </Field>
            <Field label={t("jobs.edit_dialog.to")}>
              <Input
                type="time"
                value={whTo}
                onChange={(e) => setWhTo(e.target.value)}
                className="text-base md:text-sm"
              />
            </Field>
            <Field label={t("jobs.edit_dialog.timezone")}>
              <Input
                value={whTz}
                onChange={(e) => setWhTz(e.target.value)}
                className="text-base md:text-sm"
              />
            </Field>
          </div>
          <label className="flex items-center gap-2 text-sm">
            <Switch checked={useScannerFallback} onCheckedChange={setUseScannerFallback} />
            <span>
              {t("jobs.edit_dialog.useScannerFallback")}
              <span className="block text-xs text-muted-foreground">
                {t("jobs.edit_dialog.scannerFallbackHint")}
              </span>
            </span>
          </label>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            {t("jobs.edit_dialog.cancel")}
          </Button>
          <Button disabled={!valid || createM.isPending} onClick={handleSubmit}>
            {t("jobs.edit_dialog.submit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="grid gap-1">
      <Label>{label}</Label>
      {children}
      {hint && <span className="text-xs text-muted-foreground">{hint}</span>}
    </div>
  );
}
