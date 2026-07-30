// Facebook Messenger (personal) auth wizard step.
// Collects FB cookies and forwards to the sidecar's /login endpoint.
//
// NOTE: The actual HTTP POST to the sidecar is proxied via a backend RPC
// (`channels.facebookPersonal.login`) so browsers don't need to reach the
// sidecar directly (avoids CORS and keeps the auth token server-side).
// Until that RPC lands, this component renders a read-only checklist + the
// 5 cookie fields and a Submit button that calls the RPC if available.

import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { WizardAuthStepProps } from "../channel-wizard-registry";

type CookieKey = "c_user" | "xs" | "datr" | "sb" | "fr";

const COOKIE_FIELDS: Array<{ key: CookieKey; required: boolean; placeholder: string }> = [
  { key: "c_user", required: true, placeholder: "100012345678901" },
  { key: "xs", required: true, placeholder: "43%3A..." },
  { key: "datr", required: true, placeholder: "abc123..." },
  { key: "sb", required: true, placeholder: "def456..." },
  { key: "fr", required: false, placeholder: "0abc...def (optional)" },
];

export function FBMAuthStep({ instanceId, onComplete, onSkip }: WizardAuthStepProps) {
  const { t } = useTranslation("channels");
  const [cookies, setCookies] = useState<Record<CookieKey, string>>({
    c_user: "",
    xs: "",
    datr: "",
    sb: "",
    fr: "",
  });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const allRequiredFilled = COOKIE_FIELDS
    .filter((f) => f.required)
    .every((f) => cookies[f.key].trim().length > 0);

  async function handleSubmit() {
    setSubmitting(true);
    setError(null);
    try {
      const payload = Object.fromEntries(
        Object.entries(cookies).filter(([, v]) => v.trim() !== ""),
      );
      const res = await fetch(`/api/channels/${encodeURIComponent(instanceId)}/facebook_personal/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ cookies: payload }),
      });
      if (!res.ok) {
        const body = await res.text();
        throw new Error(body || `HTTP ${res.status}`);
      }
      onComplete();
    } catch (e) {
      setError((e as Error).message || t("facebook_personal.auth.genericError"));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="rounded-md border border-amber-500 bg-amber-50 p-3 dark:bg-amber-950/20">
        <div className="font-semibold">{t("facebook_personal.warningTitle")}</div>
        <div className="mt-1 text-sm">{t("facebook_personal.warningBody")}</div>
      </div>

      <div className="space-y-2 text-sm">
        <div className="font-medium">{t("facebook_personal.auth.howtoTitle")}</div>
        <ol className="list-decimal space-y-1 pl-5">
          <li>{t("facebook_personal.auth.howtoStep1")}</li>
          <li>{t("facebook_personal.auth.howtoStep2")}</li>
          <li>{t("facebook_personal.auth.howtoStep3")}</li>
          <li>{t("facebook_personal.auth.howtoStep4")}</li>
          <li>{t("facebook_personal.auth.howtoStep5")}</li>
        </ol>
      </div>

      <div className="space-y-2">
        {COOKIE_FIELDS.map(({ key, required, placeholder }) => (
          <div key={key} className="flex flex-col gap-1">
            <label className="text-sm font-medium" htmlFor={`fbm-cookie-${key}`}>
              {key}
              {required && <span className="ml-1 text-red-500">*</span>}
            </label>
            <input
              id={`fbm-cookie-${key}`}
              type="password"
              value={cookies[key]}
              onChange={(e) => setCookies({ ...cookies, [key]: e.target.value })}
              placeholder={placeholder}
              className="rounded-md border border-gray-300 px-3 py-2 text-base md:text-sm dark:border-gray-700 dark:bg-gray-900"
            />
          </div>
        ))}
      </div>

      {error && (
        <div className="rounded-md border border-red-500 bg-red-50 p-3 text-sm dark:bg-red-950/20">
          {error}
        </div>
      )}

      <div className="flex gap-2">
        <button
          onClick={handleSubmit}
          disabled={!allRequiredFilled || submitting}
          className="rounded-md bg-blue-600 px-4 py-2 text-white hover:bg-blue-700 disabled:opacity-50"
        >
          {submitting
            ? t("facebook_personal.auth.submitting")
            : t("facebook_personal.auth.submit")}
        </button>
        <button
          onClick={onSkip}
          className="rounded-md border border-gray-300 px-4 py-2 hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-gray-800"
        >
          Skip
        </button>
      </div>
    </div>
  );
}
