import { describe, expect, it } from "vitest";
import { renderReport } from "./reports";

describe("renderReport", () => {
  it("renders markdown to HTML", () => {
    const html = renderReport("# Title\n\nSome **bold** text.");
    expect(html).toContain("<h1>Title</h1>");
    expect(html).toContain("<strong>bold</strong>");
  });

  it("rewrites relative report images to the API endpoint", () => {
    const html = renderReport("![shot](img/shot.png)");
    expect(html).toContain('src="/api/reports/img/shot.png"');
  });

  it("leaves absolute image urls alone", () => {
    const html = renderReport("![ext](https://example.com/x.png)");
    expect(html).toContain('src="https://example.com/x.png"');
  });
});
