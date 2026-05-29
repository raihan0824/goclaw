import { useTranslation } from "react-i18next";
import { Loader2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  confirmLabel?: string;
  /**
   * Optional label shown on the confirm button while `loading` is true.
   * Falls back to "..." when omitted (legacy behavior). Use this for long-
   * running confirms (e.g. LLM calls) so the user sees what's happening.
   */
  loadingLabel?: string;
  /**
   * Optional message rendered between the description and the footer while
   * `loading` is true. Useful for telegraphing latency (e.g. "this can take
   * 20-30 seconds while the agent's LLM summarizes the conversation").
   */
  loadingHint?: string;
  variant?: "default" | "destructive";
  onConfirm: () => void;
  loading?: boolean;
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  loadingLabel,
  loadingHint,
  variant = "default",
  onConfirm,
  loading,
}: ConfirmDialogProps) {
  const { t } = useTranslation("common");
  const buttonLabel = loading ? (loadingLabel ?? "...") : (confirmLabel ?? t("confirm"));
  return (
    <Dialog open={open} onOpenChange={loading ? undefined : onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        {loading && loadingHint && (
          <div className="flex items-center gap-2 rounded-md border bg-muted/40 px-3 py-2 text-sm">
            <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" />
            <span className="text-muted-foreground">{loadingHint}</span>
          </div>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={loading}>
            {t("cancel")}
          </Button>
          <Button variant={variant} onClick={onConfirm} disabled={loading} className="gap-1">
            {loading && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
            {buttonLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
