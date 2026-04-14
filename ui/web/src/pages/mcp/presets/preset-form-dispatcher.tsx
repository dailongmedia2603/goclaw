import { lazy, Suspense } from "react";
import type { MCPServerData } from "@/types/mcp";

const LarkPresetFormDialog = lazy(() =>
  import("./lark/lark-preset-form-dialog").then((m) => ({ default: m.LarkPresetFormDialog })),
);

interface PresetFormDispatcherProps {
  presetId: string | null;
  server: MCPServerData | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * Dispatches to the correct preset-specific form dialog based on presetId.
 * Returns null when no preset is active or when the preset has no dedicated form.
 */
export function PresetFormDispatcher({
  presetId,
  server,
  open,
  onOpenChange,
}: PresetFormDispatcherProps) {
  if (!open) return null;

  if (presetId === "lark") {
    return (
      <Suspense fallback={null}>
        <LarkPresetFormDialog open={open} onOpenChange={onOpenChange} server={server} />
      </Suspense>
    );
  }
  return null;
}
