import { useCallback, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Upload, FileText, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";

interface Props {
  /** Current file content (acts as a controlled textarea). */
  value: string;
  /** Called whenever the content changes — by drop, picker, or direct typing. */
  onChange: (next: string, filename?: string) => void;
  /** Max accepted bytes (UTF-8). Exceeding it shows an error and rejects the file. */
  maxBytes?: number;
  /** Browser file-picker accept hint, e.g. ".yaml,.yml,.json,.kubeconfig". */
  accept?: string;
  /** Placeholder shown in the textarea when value is empty. */
  placeholder?: string;
  /** Rows on the underlying textarea. */
  rows?: number;
}

/**
 * Drag-and-drop file content editor. The same control accepts:
 *   - a dropped file (read as text),
 *   - a file picked from the OS dialog,
 *   - direct typing/pasting into the textarea.
 *
 * Content lives in the parent (controlled). Filename is reported to the parent
 * via the second onChange arg only when the source was a file — useful for
 * auto-naming the env key.
 */
export function FileDropzone({
  value, onChange,
  maxBytes = 64 * 1024,
  accept,
  placeholder,
  rows = 6,
}: Props) {
  const { t } = useTranslation("cli-credentials");
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [dragging, setDragging] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastFile, setLastFile] = useState<string | null>(null);

  const handleFile = useCallback(async (file: File) => {
    setError(null);
    if (file.size > maxBytes) {
      setError(t("file.tooLarge", { max: maxBytes }));
      return;
    }
    try {
      const text = await file.text();
      onChange(text, file.name);
      setLastFile(file.name);
    } catch {
      setError(t("file.readError"));
    }
  }, [maxBytes, onChange, t]);

  const onDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragging(false);
    const file = e.dataTransfer.files?.[0];
    if (file) void handleFile(file);
  }, [handleFile]);

  const onDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (!dragging) setDragging(true);
  }, [dragging]);

  const onDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragging(false);
  }, []);

  const onPickerChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) void handleFile(file);
    // Reset so re-picking the same file fires onChange.
    if (inputRef.current) inputRef.current.value = "";
  }, [handleFile]);

  const clear = useCallback(() => {
    onChange("");
    setLastFile(null);
    setError(null);
  }, [onChange]);

  return (
    <div
      onDrop={onDrop}
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      onDragEnd={onDragLeave}
      className={cn(
        "grid gap-2 rounded-md border border-dashed p-2 transition-colors",
        dragging ? "border-primary bg-primary/5" : "border-input",
      )}
    >
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-xs text-muted-foreground min-w-0">
          {lastFile ? (
            <>
              <FileText className="h-3.5 w-3.5 shrink-0" />
              <span className="truncate font-mono">{lastFile}</span>
              <span className="text-muted-foreground/70">
                ({new Blob([value]).size} {t("file.bytes")})
              </span>
            </>
          ) : (
            <span>{t("file.dropHint")}</span>
          )}
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <input
            ref={inputRef}
            type="file"
            accept={accept}
            onChange={onPickerChange}
            className="hidden"
          />
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-xs gap-1"
            onClick={() => inputRef.current?.click()}
          >
            <Upload className="h-3.5 w-3.5" />
            {t("file.upload")}
          </Button>
          {value && (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={clear}
              title={t("file.clear")}
            >
              <X className="h-3.5 w-3.5" />
            </Button>
          )}
        </div>
      </div>

      <Textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        rows={rows}
        className="text-base md:text-sm font-mono resize-y"
      />

      {error && <p className="text-xs text-destructive">{error}</p>}
    </div>
  );
}
