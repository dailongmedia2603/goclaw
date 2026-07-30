import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus, RotateCw, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { EmptyState } from "@/components/shared/empty-state";
import {
  type CredentialStatus,
  type FBCloakCredential,
  useAddCredential,
  useDeleteCredential,
  useFBCloakCredentials,
  useTestCredential,
} from "@/hooks/use-fbcloak";

export function CredentialsTab() {
  const { t } = useTranslation("fbcloak");
  const { data: credentials = [], isLoading } = useFBCloakCredentials();
  const [showAdd, setShowAdd] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<FBCloakCredential | null>(null);
  const testM = useTestCredential();
  const deleteM = useDeleteCredential();

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Dialog open={showAdd} onOpenChange={setShowAdd}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="h-4 w-4 mr-2" />
              {t("credentials.addButton")}
            </Button>
          </DialogTrigger>
          <AddCredentialDialog onClose={() => setShowAdd(false)} />
        </Dialog>
      </div>

      {!isLoading && credentials.length === 0 ? (
        <EmptyState title={t("credentials.title")} description={t("credentials.empty")} />
      ) : (
        <div className="overflow-x-auto rounded-md border">
          <table className="min-w-[600px] w-full text-sm">
            <thead className="bg-muted/50 text-left">
              <tr>
                <th className="px-3 py-2">{t("credentials.columns.fanpage")}</th>
                <th className="px-3 py-2">{t("credentials.columns.status")}</th>
                <th className="px-3 py-2">{t("credentials.columns.lastCheck")}</th>
                <th className="px-3 py-2">{t("credentials.columns.actions")}</th>
              </tr>
            </thead>
            <tbody>
              {credentials.map((c) => (
                <tr key={c.id} className="border-t">
                  <td className="px-3 py-2">
                    <div className="font-medium">{c.fanpageName}</div>
                    <div className="text-xs text-muted-foreground">{c.fanpageId}</div>
                  </td>
                  <td className="px-3 py-2">
                    <CredentialStatusBadge status={c.status} />
                  </td>
                  <td className="px-3 py-2 text-xs text-muted-foreground">
                    {c.lastCheckAt ? new Date(c.lastCheckAt).toLocaleString() : "—"}
                  </td>
                  <td className="px-3 py-2">
                    <div className="flex gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={testM.isPending}
                        onClick={() => testM.mutate(c.id)}
                      >
                        <RotateCw className="h-4 w-4 mr-1" />
                        {testM.isPending ? t("credentials.testing") : t("credentials.test")}
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setDeleteTarget(c)}
                      >
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

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
        title={t("credentials.delete")}
        description={t("credentials.confirmDelete")}
        confirmLabel={t("credentials.delete")}
        variant="destructive"
        onConfirm={async () => {
          if (deleteTarget) await deleteM.mutateAsync(deleteTarget.id);
          setDeleteTarget(null);
        }}
      />
    </div>
  );
}

function CredentialStatusBadge({ status }: { status: CredentialStatus }) {
  const { t } = useTranslation("fbcloak");
  const variant: "default" | "secondary" | "outline" | "destructive" =
    status === "active" ? "default"
    : status === "checkpoint" || status === "expired" ? "destructive"
    : "secondary";
  return <Badge variant={variant}>{t(`credentials.status.${status}`)}</Badge>;
}

function AddCredentialDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation("fbcloak");
  const addM = useAddCredential();
  const [fanpageId, setFanpageId] = useState("");
  const [fanpageName, setFanpageName] = useState("");
  const [cookies, setCookies] = useState("");
  const [proxyUrl, setProxyUrl] = useState("");
  const [userAgent, setUserAgent] = useState("");

  const handleSubmit = async () => {
    await addM.mutateAsync({
      fanpageId: fanpageId.trim(),
      fanpageName: fanpageName.trim(),
      cookies,
      proxyUrl: proxyUrl.trim() || undefined,
      userAgent: userAgent.trim() || undefined,
    });
    onClose();
  };

  const valid = fanpageId.trim() && fanpageName.trim() && cookies.trim();

  return (
    <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{t("credentials.addDialog.title")}</DialogTitle>
      </DialogHeader>
      <div className="grid gap-3">
        <div className="grid gap-1">
          <Label htmlFor="fb-id">{t("credentials.addDialog.fanpageId")}</Label>
          <Input
            id="fb-id"
            value={fanpageId}
            onChange={(e) => setFanpageId(e.target.value)}
            className="text-base md:text-sm"
          />
        </div>
        <div className="grid gap-1">
          <Label htmlFor="fb-name">{t("credentials.addDialog.fanpageName")}</Label>
          <Input
            id="fb-name"
            value={fanpageName}
            onChange={(e) => setFanpageName(e.target.value)}
            className="text-base md:text-sm"
          />
        </div>
        <div className="grid gap-1">
          <Label htmlFor="fb-cookies">{t("credentials.addDialog.cookies")}</Label>
          <Textarea
            id="fb-cookies"
            value={cookies}
            onChange={(e) => setCookies(e.target.value)}
            placeholder={t("credentials.addDialog.cookiesPlaceholder")}
            rows={6}
            className="text-base md:text-sm font-mono"
          />
        </div>
        <div className="grid gap-1">
          <Label htmlFor="fb-proxy">{t("credentials.addDialog.proxyUrl")}</Label>
          <Input
            id="fb-proxy"
            value={proxyUrl}
            onChange={(e) => setProxyUrl(e.target.value)}
            placeholder={t("credentials.addDialog.proxyUrlPlaceholder")}
            className="text-base md:text-sm"
          />
        </div>
        <div className="grid gap-1">
          <Label htmlFor="fb-ua">{t("credentials.addDialog.userAgent")}</Label>
          <Input
            id="fb-ua"
            value={userAgent}
            onChange={(e) => setUserAgent(e.target.value)}
            className="text-base md:text-sm"
          />
        </div>
      </div>
      <DialogFooter>
        <Button variant="outline" onClick={onClose}>
          {t("credentials.addDialog.cancel")}
        </Button>
        <Button disabled={!valid || addM.isPending} onClick={handleSubmit}>
          {t("credentials.addDialog.submit")}
        </Button>
      </DialogFooter>
    </DialogContent>
  );
}
