// Best-effort interpretation of raw opencode event objects.
//
// Real schema (opencode 1.x): every line is {type, timestamp, part:{...}};
// assistant output arrives as type "text" / "reasoning" parts and tool
// activity as tool events with part.state.{input,output}. Legacy
// message/tool_use shapes are kept as fallbacks for old recordings.

export type AnyEvent = Record<string, unknown>;

export type TranscriptSection = {
  label: string;
  text: string;
  tone?: "del" | "ins";
};

export type TranscriptItem =
  | { kind: "text"; text: string }
  | { kind: "reasoning"; text: string }
  | {
      kind: "tool";
      name: string;
      title: string;
      sections: TranscriptSection[];
    };

function asObject(e: unknown): AnyEvent | null {
  return e !== null && typeof e === "object" ? (e as AnyEvent) : null;
}

export function eventType(e: unknown): string {
  const o = asObject(e);
  return o && typeof o.type === "string" ? o.type : "";
}

export function isMessage(e: unknown): boolean {
  return eventType(e).includes("message");
}

export function isTool(e: unknown): boolean {
  return eventType(e).includes("tool");
}

function part(e: unknown): AnyEvent | null {
  const o = asObject(e);
  return o ? asObject(o.part) : null;
}

function partType(e: unknown): string {
  const p = part(e);
  return p && typeof p.type === "string" ? p.type : "";
}

function str(v: unknown): string {
  return typeof v === "string" ? v : "";
}

const MAX_SECTION = 4000;

function clip(s: string): string {
  return s.length > MAX_SECTION ? s.slice(0, MAX_SECTION) + "\n… truncated" : s;
}

/** Pull human-readable text out of a message-shaped event, if any. */
export function extractText(e: unknown): string {
  const o = asObject(e);
  if (!o) return "";
  const parts: string[] = [];
  const visit = (v: unknown, depth: number) => {
    if (depth > 4 || v === null || v === undefined) return;
    if (typeof v === "string") {
      parts.push(v);
      return;
    }
    if (Array.isArray(v)) {
      for (const item of v) {
        const io = asObject(item);
        if (io && typeof io.text === "string") parts.push(io.text);
        else if (typeof item === "string") parts.push(item);
      }
      return;
    }
    const vo = asObject(v);
    if (!vo) return;
    if (typeof vo.text === "string") {
      parts.push(vo.text);
      return;
    }
    if ("content" in vo) visit(vo.content, depth + 1);
    else if ("part" in vo) visit(vo.part, depth + 1);
    else if ("message" in vo) visit(vo.message, depth + 1);
  };
  for (const key of ["text", "content", "part", "message"]) {
    if (key in o) visit(o[key], 0);
    if (parts.length > 0) break;
  }
  return parts.join("\n").trim();
}

/** Name to show for a tool event: tool name if present, else its type. */
export function toolName(e: unknown): string {
  const o = asObject(e);
  if (!o) return "tool";
  for (const key of ["name", "tool", "tool_name"]) {
    if (typeof o[key] === "string" && o[key] !== "") return o[key] as string;
  }
  const p = part(e);
  if (p) {
    for (const key of ["tool", "name"]) {
      if (typeof p[key] === "string" && p[key] !== "") return p[key] as string;
    }
  }
  return eventType(e) || "tool";
}

export function pretty(e: unknown): string {
  try {
    return JSON.stringify(e, null, 2);
  } catch {
    return String(e);
  }
}

function toolItem(e: unknown): TranscriptItem {
  const name = toolName(e);
  const p = part(e) ?? asObject(e) ?? {};
  const state = asObject(p.state) ?? {};
  const input =
    asObject(state.input) ??
    asObject(p.args) ??
    asObject((asObject(e) ?? {}).input) ??
    {};
  const output = clip(str(state.output));
  const filePath = str(input.filePath) || str(input.path);
  const sections: TranscriptSection[] = [];
  let title = filePath;

  switch (name) {
    case "bash": {
      const cmd = str(input.command);
      title = cmd.split("\n")[0].slice(0, 80);
      if (cmd) sections.push({ label: "command", text: clip(cmd) });
      if (output) sections.push({ label: "output", text: output });
      break;
    }
    case "edit": {
      const oldS = str(input.oldString);
      const newS = str(input.newString);
      if (oldS)
        sections.push({ label: "removed", text: clip(oldS), tone: "del" });
      if (newS)
        sections.push({ label: "added", text: clip(newS), tone: "ins" });
      break;
    }
    case "write": {
      const content = str(input.content);
      if (content) sections.push({ label: "content", text: clip(content) });
      break;
    }
    case "read":
      break; // path in the title is enough
    default: {
      sections.push({ label: "data", text: clip(pretty(e)) });
      break;
    }
  }
  if (sections.length === 0 && !title) {
    sections.push({ label: "data", text: clip(pretty(e)) });
  }
  return { kind: "tool", name, title, sections };
}

/** Interpret a raw event stream as renderable transcript items. */
export function parseItems(events: unknown[]): TranscriptItem[] {
  const items: TranscriptItem[] = [];
  for (const e of events) {
    const t = eventType(e);
    const pt = partType(e);
    if (isTool(e)) {
      items.push(toolItem(e));
    } else if (t === "text" || pt === "text") {
      const text = str(part(e)?.text) || extractText(e);
      if (text) items.push({ kind: "text", text });
    } else if (t === "reasoning" || pt === "reasoning") {
      const text = str(part(e)?.text) || extractText(e);
      if (text) items.push({ kind: "reasoning", text });
    } else if (isMessage(e)) {
      const text = extractText(e);
      if (text) items.push({ kind: "text", text });
    }
  }
  return items;
}

/** Last assistant text in the stream — used by Compare. */
export function lastAssistantText(events: unknown[]): string {
  const items = parseItems(events);
  for (let i = items.length - 1; i >= 0; i--) {
    const item = items[i];
    if (item.kind === "text") return item.text;
  }
  return "";
}

/** Case-insensitive filter over every string an item renders. */
export function itemMatches(item: TranscriptItem, query: string): boolean {
  if (!query) return true;
  const q = query.toLowerCase();
  if (item.kind === "text" || item.kind === "reasoning") {
    return item.text.toLowerCase().includes(q);
  }
  if (item.name.toLowerCase().includes(q)) return true;
  if (item.title.toLowerCase().includes(q)) return true;
  return item.sections.some((s) => s.text.toLowerCase().includes(q));
}
