// Facebook Messenger (personal) re-authentication dialog.
// Rendered from the channel detail page when cookies expire.

import { useTranslation } from "react-i18next";

import type { ReauthDialogProps } from "../channel-wizard-registry";

import { FBMAuthStep } from "./fbm-auth-step";

export function FBMReauthDialog({ open, onOpenChange, instanceId, instanceName, onSuccess }: ReauthDialogProps) {
  const { t } = useTranslation("channels");

  if (!open) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={() => onOpenChange(false)}
    >
      <div
        className="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-lg bg-white p-6 shadow-xl dark:bg-gray-900"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-4">
          <h2 className="text-lg font-semibold">
            {t("facebook_personal.reauth.title", { name: instanceName })}
          </h2>
          <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">
            {t("facebook_personal.reauth.description")}
          </p>
        </div>
        <FBMAuthStep
          instanceId={instanceId}
          onComplete={() => {
            onSuccess();
            onOpenChange(false);
          }}
          onSkip={() => onOpenChange(false)}
        />
      </div>
    </div>
  );
}
