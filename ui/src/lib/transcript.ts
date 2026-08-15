// Best-effort interpretation of raw opencode event objects.

export type AnyEvent = Record<string, unknown>;

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
  const part = asObject(o.part);
  if (part) {
    for (const key of ["tool", "name"]) {
      if (typeof part[key] === "string" && part[key] !== "")
        return part[key] as string;
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

/** Last message event with extractable text — used by Compare. */
export function lastAssistantText(events: unknown[]): string {
  for (let i = events.length - 1; i >= 0; i--) {
    if (!isMessage(events[i])) continue;
    const text = extractText(events[i]);
    if (text !== "") return text;
  }
  return "";
}
