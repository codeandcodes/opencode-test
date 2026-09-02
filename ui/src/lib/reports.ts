import { marked } from "marked";

// renderReport turns a report's markdown into HTML and points relative
// image references (img/...) at the server's report-image endpoint.
export function renderReport(md: string): string {
  const html = marked.parse(md, { async: false }) as string;
  return html.replaceAll('src="img/', 'src="/api/reports/img/');
}
