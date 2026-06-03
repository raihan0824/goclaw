import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { UserPickerCombobox } from "@/components/shared/user-picker-combobox";
import { AgentSelector } from "@/components/chat/agent-selector";

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
  agent: string;
}

function rowsFromValue(v: Record<string, string> | undefined): Row[] {
  const out: Row[] = [];
  if (v) {
    for (const [jid, agent] of Object.entries(v)) {
      out.push({ jid, agent });
    }
  }
  out.push({ jid: "", agent: "" });
  return out;
}

function rowsToMap(rows: Row[]): Record<string, string> | undefined {
  const map: Record<string, string> = {};
  for (const r of rows) {
    const jid = r.jid.trim();
    const agent = r.agent.trim();
    if (jid !== "" && agent !== "") {
      map[jid] = agent;
    }
  }
  return Object.keys(map).length > 0 ? map : undefined;
}

/**
 * Row-based editor for the WhatsApp per-group agent override map.
 *
 * Each row: a group JID picker on the left, an agent selector on the right.
 * Same local-state-as-source-of-truth pattern as WhatsappGroupAliasesField:
 * the parent map is committed on every keystroke, but the local row list
 * survives half-filled rows that would otherwise be filtered out by
 * rowsToMap and lost on the next re-render.
 */
export function WhatsappGroupAgentOverridesField({ value, onChange, label, help }: Props) {
  const { t } = useTranslation("channels");

  const [rows, setRows] = useState<Row[]>(() => rowsFromValue(value));
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
    (idx: number, field: "jid" | "agent", v: string) => {
      const next = rows.map((r, i) => (i === idx ? { ...r, [field]: v } : r));
      commit(next);
    },
    [rows, commit],
  );

  const removeRow = useCallback(
    (idx: number) => {
      const next = rows.filter((_, i) => i !== idx);
      if (next.length === 0) next.push({ jid: "", agent: "" });
      commit(next);
    },
    [rows, commit],
  );

  const addRow = useCallback(() => {
    commit([...rows, { jid: "", agent: "" }]);
  }, [rows, commit]);

  // Duplicate-JID detection across non-empty rows.
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
                  placeholder={t("groupAgentOverrides.jidPlaceholder")}
                />
                {isDup && (
                  <p className="text-xs text-destructive mt-0.5">
                    {t("groupAgentOverrides.duplicateJid")}
                  </p>
                )}
              </div>
              <div className="flex-1 min-w-0">
                <AgentSelector
                  value={row.agent}
                  onChange={(v) => setRow(idx, "agent", v)}
                />
              </div>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="mt-0.5 h-8 w-8 shrink-0"
                onClick={() => removeRow(idx)}
                aria-label={t("groupAgentOverrides.removeRow")}
              >
                <X className="h-4 w-4" />
              </Button>
            </div>
          );
        })}

        <Button type="button" variant="outline" size="sm" onClick={addRow} className="w-fit gap-1">
          <Plus className="h-3.5 w-3.5" /> {t("groupAgentOverrides.addRow")}
        </Button>
      </div>

      {help && <p className="text-xs text-muted-foreground">{help}</p>}
    </div>
  );
}

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
