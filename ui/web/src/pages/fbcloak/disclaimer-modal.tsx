import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { useAckDisclaimer, useDisclaimerStatus } from "@/hooks/use-fbcloak";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

// DisclaimerModal — shown the first time a tenant lands on /fbcloak/jobs
// (or when they try to enable a job and the server returns
// MsgFBCloakDisclaimerRequired). Three checkboxes — all required — guard
// the submit button so accidental clicks don't ack on behalf of the
// tenant.
export function DisclaimerModal({ open, onOpenChange }: Props) {
  const { t } = useTranslation("fbcloak");
  const { data: status } = useDisclaimerStatus();
  const ack = useAckDisclaimer();

  const [c1, setC1] = useState(false);
  const [c2, setC2] = useState(false);
  const [c3, setC3] = useState(false);
  const allChecked = c1 && c2 && c3;

  const handleSubmit = async () => {
    await ack.mutateAsync(status?.currentVersion);
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("disclaimer.title")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 text-sm">
          <p className="text-muted-foreground">{t("disclaimer.intro")}</p>
          <div className="space-y-3 rounded border p-3">
            <label className="flex items-start gap-2 cursor-pointer">
              <Switch checked={c1} onCheckedChange={setC1} />
              <span>{t("disclaimer.checkbox1")}</span>
            </label>
            <label className="flex items-start gap-2 cursor-pointer">
              <Switch checked={c2} onCheckedChange={setC2} />
              <span>{t("disclaimer.checkbox2")}</span>
            </label>
            <label className="flex items-start gap-2 cursor-pointer">
              <Switch checked={c3} onCheckedChange={setC3} />
              <span>{t("disclaimer.checkbox3")}</span>
            </label>
          </div>
          <div className="text-xs text-muted-foreground">
            {t("disclaimer.version", { version: status?.currentVersion ?? "v1.0" })}
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("disclaimer.cancel")}
          </Button>
          <Button disabled={!allChecked || ack.isPending} onClick={handleSubmit}>
            {t("disclaimer.submit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
