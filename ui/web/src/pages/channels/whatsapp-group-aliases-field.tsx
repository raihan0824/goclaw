import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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

// rowsFromValue turns the committed map into an ordered list, appending one
// trailing empty row so the admin can always start typing without clicking
// "Add group" first.
function rowsFromValue(v: Record<string, string> | undefined): Row[] {
  const out: Row[] = [];
  if (v) {
    for (const [jid, name] of Object.entries(v)) {
      out.push({ jid, name });
    }
  }
  out.push({ jid: "", name: "" });
  return out;
}

// rowsToMap serializes the editor's row list back to the map shape. Only
// rows with both fields populated are persisted — empty rows are working
// state, not data.
function rowsToMap(rows: Row[]): Record<string, string> | undefined {
  const map: Record<string, string> = {};
  for (const r of rows) {
    const jid = r.jid.trim();
    const name = r.name.trim();
    if (jid !== "" && name !== "") {
      map[jid] = name;
    }
  }
  return Object.keys(map).length > 0 ? map : undefined;
}

/**
 * Row-based editor for WhatsApp group alias map (JID → display name).
 *
 * The trick: rows live in **local state**, not derived from `value`. If we
 * derived them, half-filled rows (e.g. name typed but JID still blank)
 * would round-trip through the parent's filtered map and lose their text.
 * The map is committed to the parent on every keystroke, but the local row
 * list is the source of truth for what's visible.
 *
 * External updates to `value` (initial load, form reset) sync into local
 * state via the lastEmittedRef check — we only overwrite local rows when
 * the incoming map differs from what we last emitted, so the parent's
 * onChange echo doesn't clobber in-flight edits.
 */
export function WhatsappGroupAliasesField({ value, onChange, label, help }: Props) {
  const { t } = useTranslation("channels");

  const [rows, setRows] = useState<Row[]>(() => rowsFromValue(value));
  // Snapshot of the last map we emitted to the parent. When the parent
  // bounces it back via `value`, we compare; only an external change (init
  // load, reset, server hydration) should force-resync the rows.
  const lastEmittedRef = useRef<Record<string, string> | undefined>(value);

  useEffect(() => {
    if (!mapsEqual(value, lastEmittedRef.current)) {
      setRows(rowsFromValue(value));
      lastEmittedRef.current = value;
    }
  }, [value]);

  const commit = useCallback(
    (next: Row[]) => {
      setRows(next);
      const map = rowsToMap(next);
      lastEmittedRef.current = map;
      onChange(map);
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
      const next = rows.filter((_, i) => i !== idx);
      // Always keep at least one empty trailing row so the editor never
      // shows zero inputs and force the admin to click "Add group" first.
      if (next.length === 0) next.push({ jid: "", name: "" });
      commit(next);
    },
    [rows, commit],
  );

  const addRow = useCallback(() => {
    commit([...rows, { jid: "", name: "" }]);
  }, [rows, commit]);

  // Duplicate-JID detection across non-empty rows — inline UX warning only.
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

// mapsEqual returns true when two maps have identical key sets and values.
// Cheap enough for the dozen-entry max we expect on this field.
function mapsEqual(
  a: Record<string, string> | undefined,
  b: Record<string, string> | undefined,
): boolean {
  if (a === b) return true;
  if (!a || !b) return false;
  const ka = Object.keys(a);
  const kb = Object.keys(b);
  if (ka.length !== kb.length) return false;
  for (const k of ka) {
    if (a[k] !== b[k]) return false;
  }
  return true;
}
