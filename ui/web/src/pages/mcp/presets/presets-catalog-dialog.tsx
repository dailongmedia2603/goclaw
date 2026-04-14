import { useTranslation } from "react-i18next";
import { Wrench, Loader2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { usePresets } from "./use-presets";
import type { PresetMetadata } from "./types";

interface PresetsCatalogDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSelect: (presetId: string | "__custom__") => void;
}

export function PresetsCatalogDialog({ open, onOpenChange, onSelect }: PresetsCatalogDialogProps) {
  const { t } = useTranslation("mcp");
  const { data: presets = [], isLoading } = usePresets();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("presets.catalog.title")}</DialogTitle>
          <DialogDescription>{t("presets.catalog.description")}</DialogDescription>
        </DialogHeader>

        {isLoading ? (
          <div className="flex items-center justify-center py-10 text-muted-foreground">
            <Loader2 className="h-5 w-5 animate-spin" />
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 py-2">
            {presets.map((p) => (
              <PresetCard
                key={p.id}
                preset={p}
                onSelect={() => {
                  onSelect(p.id);
                  onOpenChange(false);
                }}
              />
            ))}
            <CustomCard
              onSelect={() => {
                onSelect("__custom__");
                onOpenChange(false);
              }}
            />
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

function PresetCard({ preset, onSelect }: { preset: PresetMetadata; onSelect: () => void }) {
  const { t } = useTranslation("mcp");
  return (
    <button
      type="button"
      onClick={onSelect}
      data-testid={`preset-card-${preset.id}`}
      className="group flex flex-col items-start gap-2 rounded-lg border bg-card p-4 text-left transition hover:border-primary hover:bg-accent/30 focus:outline-none focus:ring-2 focus:ring-ring"
    >
      <div className="flex items-center gap-2 w-full">
        {preset.icon ? (
          <img
            src={preset.icon}
            alt=""
            className="h-8 w-8 shrink-0 text-muted-foreground"
            aria-hidden
          />
        ) : (
          <div className="h-8 w-8 shrink-0 rounded bg-muted" />
        )}
        <span className="font-medium text-sm">{preset.display_name}</span>
      </div>
      <p className="text-xs text-muted-foreground line-clamp-3">{preset.description}</p>
      <span className="mt-auto inline-flex items-center text-xs font-medium text-primary opacity-0 group-hover:opacity-100 transition">
        {t("presets.catalog.select")} →
      </span>
    </button>
  );
}

function CustomCard({ onSelect }: { onSelect: () => void }) {
  const { t } = useTranslation("mcp");
  return (
    <button
      type="button"
      onClick={onSelect}
      data-testid="preset-card-custom"
      className="group flex flex-col items-start gap-2 rounded-lg border border-dashed bg-transparent p-4 text-left transition hover:border-primary hover:bg-accent/20 focus:outline-none focus:ring-2 focus:ring-ring"
    >
      <div className="flex items-center gap-2 w-full">
        <Wrench className="h-7 w-7 shrink-0 text-muted-foreground" aria-hidden />
        <span className="font-medium text-sm">{t("presets.custom.title")}</span>
      </div>
      <p className="text-xs text-muted-foreground line-clamp-3">{t("presets.custom.description")}</p>
    </button>
  );
}

// Re-export Button so tests can tree-shake imports cleanly if needed later.
export { Button };
