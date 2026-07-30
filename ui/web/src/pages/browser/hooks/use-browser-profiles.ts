import { useState, useCallback } from "react";
import { useHttp, useWs } from "@/hooks/use-ws";
import { toast } from "@/stores/use-toast-store";
import i18next from "i18next";
import { Methods } from "@/api/protocol";
import { userFriendlyError } from "@/lib/error-utils";

export interface BrowserProfile {
  name: string;
  running: boolean;
  tabs: number;
  shared: boolean;
  domains: string[];
  vnc_url: string;
  active: boolean; // true = loaded in registry, false = config-only (needs restart)
}

interface ProfilesResponse {
  profiles: BrowserProfile[];
}

interface ConfigGetResponse {
  config: Record<string, unknown>;
  hash: string;
}

export function useBrowserProfiles() {
  const http = useHttp();
  const ws = useWs();
  const [profiles, setProfiles] = useState<BrowserProfile[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const res = await http.get<ProfilesResponse>("/v1/browser/profiles");
      setProfiles(res.profiles ?? []);
    } catch {
      setProfiles([]);
    } finally {
      setLoading(false);
    }
  }, [http]);

  const saveProfiles = useCallback(
    async (profilesConfig: Record<string, ProfileConfig>, defaultProfile?: string) => {
      setSaving(true);
      try {
        // Get current config + hash for optimistic concurrency
        const current = await ws.call<ConfigGetResponse>(Methods.CONFIG_GET, {});
        const patch: Record<string, unknown> = {
          tools: {
            browser: {
              enabled: true,
              profiles: profilesConfig,
              ...(defaultProfile ? { default_profile: defaultProfile } : {}),
            },
          },
        };
        await ws.call(Methods.CONFIG_PATCH, {
          raw: JSON.stringify(patch),
          baseHash: current.hash,
        });
        toast.success(i18next.t("browser:toast.saved"));
        // Refresh after config change
        setTimeout(() => refresh(), 1000);
      } catch (err) {
        toast.error(i18next.t("browser:toast.saveFailed"), userFriendlyError(err));
        throw err;
      } finally {
        setSaving(false);
      }
    },
    [ws, refresh],
  );

  return { profiles, loading, saving, refresh, saveProfiles };
}

export interface ProfileConfig {
  remote_url?: string;
  headless?: boolean;
  shared?: boolean;
  domains?: string[];
  vnc_url?: string;
  action_timeout_ms?: number;
  idle_timeout_ms?: number;
  max_pages?: number;
}
