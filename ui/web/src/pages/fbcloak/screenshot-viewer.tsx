import { useEffect, useState } from "react";
import { Skeleton } from "@/components/ui/skeleton";
import { useAuthStore } from "@/stores/use-auth-store";
import { useScreenshotPath } from "@/hooks/use-fbcloak";

interface Props {
  sendLogId: string;
  kind: "pre" | "post";
  alt?: string;
}

// ScreenshotViewer fetches the on-disk path via fbcloak.log.screenshot,
// then signs it through POST /v1/files/sign so the rendered <img> uses a
// short-lived ?ft= URL (no Bearer token leak in img src). The signed URL
// is bound to (path, key) — tenant isolation is upstream in the RPC.
export function ScreenshotViewer({ sendLogId, kind, alt }: Props) {
  const fetchPath = useScreenshotPath();
  const token = useAuthStore((s) => s.token);
  const [signedUrl, setSignedUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setSignedUrl(null);
      setError(null);
      try {
        const path = await fetchPath(sendLogId, kind);
        if (!path) {
          setError("no-screenshot");
          return;
        }
        const res = await fetch("/v1/files/sign", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: token ? `Bearer ${token}` : "",
          },
          body: JSON.stringify({ path }),
        });
        if (!res.ok) {
          setError(`sign-failed:${res.status}`);
          return;
        }
        const { url } = (await res.json()) as { url: string };
        if (!cancelled) setSignedUrl(url);
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : "load-failed");
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [sendLogId, kind, fetchPath, token]);

  if (error === "no-screenshot") return null;
  if (error) {
    return (
      <div className="text-xs text-destructive">screenshot error: {error}</div>
    );
  }
  if (!signedUrl) {
    return <Skeleton className="h-32 w-full" />;
  }
  return (
    <a href={signedUrl} target="_blank" rel="noreferrer" className="block">
      <img
        src={signedUrl}
        alt={alt ?? `screenshot-${kind}`}
        className="max-h-64 w-auto rounded border"
      />
    </a>
  );
}
