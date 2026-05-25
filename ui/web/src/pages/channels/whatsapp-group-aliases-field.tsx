import { useCallback, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { UserPickerCombobox } from "@/components/shared/user-picker-combobox";

interface Props {
  value: Record<string, string> | undefined;
  onChange: (next: Record<string, string> | undefined) => void;
  /** Field label rendered above the editor. */
  label: string;
  /** Help text rendered under the editor. */
  help?: string;
}

interface Row {
  jid: string;
  name: string;
}

/**
 * Row-based editor for WhatsApp group alias map (JID → display name).
 *
 * Two columns:
 *   - Group picker (filtered to known group contacts, free-form fallback)
 *   - Display name text input
 *
 * Empty rows are tolerated in local state but stripped on every change before
 * notifying the parent. Duplicate JIDs in the UI are flagged inline; the
 * onChange map naturally dedupes (last-write-wins) so persistence is
 * deterministic.
 */
export function WhatsappGroupAliasesField({ value, onChange, label, help }: Props) {
  const { t } = useTranslation("channels");

  // Convert the controlled map into an ordered row list for editing.
  // Add one trailing empty row so the admin can always start typing.
  const rows: Row[] = useMemo(() => {
    const out: Row[] = [];
    if (value) {
      for (const [jid, name] of Object.entries(value)) {
        out.push({ jid, name });
      }
    }
    out.push({ jid: "", name: "" });
    return out;
  }, [value]);

  // Push the current row list back up as a clean map (skipping empty rows).
  // Last value wins on duplicate JIDs — matches server-side semantics.
  const commit = useCallback(
    (next: Row[]) => {
      const map: Record<string, string> = {};
      for (const r of next) {
        const jid = r.jid.trim();
        const name = r.name.trim();
        if (jid !== "" && name !== "") {
          map[jid] = name;
        }
      }
      onChange(Object.keys(map).length > 0 ? map : undefined);
    },
    [onChange],
  );

  const setRow = useCallback(
    (idx: number, field: "jid" | "name", v: string) => {
      const next = rows.map((r, i) => (i === idx ? { ...r, [field]: v } : r));
      commit(next);
    },
    [rows, commit],
  );

  const removeRow = useCallback(
    (idx: number) => {
      commit(rows.filter((_, i) => i !== idx));
    },
    [rows, commit],
  );

  const addRow = useCallback(() => {
    commit([...rows, { jid: "", name: "" }]);
  }, [rows, commit]);

  // Detect duplicate JIDs across non-empty rows so the admin sees an inline
  // warning before they hit save.
  const dupes = useMemo(() => {
    const seen = new Set<string>();
    const out = new Set<string>();
    for (const r of rows) {
      const jid = r.jid.trim();
      if (jid === "") continue;
      if (seen.has(jid)) out.add(jid);
      seen.add(jid);
    }
    return out;
  }, [rows]);

  return (
    <div className="grid gap-1.5">
      <Label>{label}</Label>

      <div className="grid gap-2 rounded-md border p-3">
        {rows.map((row, idx) => {
          const jid = row.jid.trim();
          const isDup = jid !== "" && dupes.has(jid);
          return (
            <div key={idx} className="flex items-start gap-2">
              <div className="flex-1 min-w-0">
                <UserPickerCombobox
                  value={row.jid}
                  onChange={(v) => setRow(idx, "jid", v)}
                  onSelect={(v) => setRow(idx, "jid", v)}
                  peerKind="group"
                  allowCustom
                  placeholder={t("groupAliases.jidPlaceholder")}
                />
                {isDup && (
                  <p className="text-xs text-destructive mt-0.5">
                    {t("groupAliases.duplicateJid")}
                  </p>
                )}
              </div>
              <div className="flex-1 min-w-0">
                <Input
                  value={row.name}
                  onChange={(e) => setRow(idx, "name", e.target.value)}
                  placeholder={t("groupAliases.namePlaceholder")}
                  className="text-base md:text-sm"
                />
              </div>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="mt-0.5 h-8 w-8 shrink-0"
                onClick={() => removeRow(idx)}
                aria-label={t("groupAliases.removeRow")}
              >
                <X className="h-4 w-4" />
              </Button>
            </div>
          );
        })}

        <Button type="button" variant="outline" size="sm" onClick={addRow} className="w-fit gap-1">
          <Plus className="h-3.5 w-3.5" /> {t("groupAliases.addRow")}
        </Button>
      </div>

      {help && <p className="text-xs text-muted-foreground">{help}</p>}
    </div>
  );
}
